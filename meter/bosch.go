package meter

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/util"
	"github.com/evcc-io/evcc/util/request"
)

// Bosch is the Bosch BPT-S 5 Hybrid meter
type BoschApiClient struct {
	*request.Helper
	uri, wuSid             string
	currentBatterySocValue float64
	einspeisung            float64
	strombezugAusNetz      float64
	pvLeistungWatt         float64
	batterieLadeStrom      float64
	verbrauchVonBatterie   float64
}

type Bosch struct {
	usage                   string
	currentErr              error
	currentTotalEnergyValue float64
	requestClient           *BoschApiClient
}

var boschInstance *BoschApiClient = nil

func init() {
	registry.Add("bosch", NewBoschFromConfig)
}

//go:generate go run ../cmd/tools/decorate.go -f decorateBosch -b api.Meter -t "api.MeterEnergy,TotalEnergy,func() (float64, error)" -t "api.Battery,SoC,func() (float64, error)"

// NewBoschFromConfig creates a Bosch BPT-S 5 Hybrid Meter from generic config
func NewBoschFromConfig(other map[string]interface{}) (api.Meter, error) {
	cc := struct {
		URI, Usage string
	}{}

	if err := util.DecodeOther(other, &cc); err != nil {
		return nil, err
	}

	if cc.Usage == "" {
		return nil, errors.New("missing usage")
	}

	_, err := url.Parse(cc.URI)
	if err != nil {
		return nil, fmt.Errorf("%s is invalid: %s", cc.URI, err)
	}

	return NewBosch(cc.URI, cc.Usage)
}

// NewBosch creates a Bosch Meter
func NewBosch(uri, usage string) (api.Meter, error) {
	log := util.NewLogger("bosch")

	if boschInstance == nil {
		boschInstance = &BoschApiClient{
			Helper:                 request.NewHelper(log),
			uri:                    util.DefaultScheme(strings.TrimSuffix(uri, "/"), "http"),
			currentBatterySocValue: 0.0,
			einspeisung:            0.0,
			strombezugAusNetz:      0.0,
			pvLeistungWatt:         0.0,
			batterieLadeStrom:      0.0,
			verbrauchVonBatterie:   0.0,
		}

		// ignore the self signed certificate
		boschInstance.Client.Transport = request.NewTripper(log, request.InsecureTransport())
		// create cookie jar to save login tokens
		boschInstance.Client.Jar, _ = cookiejar.New(nil)

		if err := boschInstance.Login(); err != nil {
			return nil, err
		}

		go readLoop(boschInstance)
	}

	m := &Bosch{
		usage:                   strings.ToLower(usage),
		currentErr:              nil,
		currentTotalEnergyValue: 0.0,
		requestClient:           boschInstance,
	}

	// decorate api.MeterEnergy
	var totalEnergy func() (float64, error)
	if m.usage == "grid" || m.usage == "pv" {
		totalEnergy = m.totalEnergy
	}

	// decorate api.BatterySoC
	var batterySoC func() (float64, error)
	if usage == "battery" {
		batterySoC = m.batterySoC
	}

	return decorateBosch(m, totalEnergy, batterySoC), nil
}

// Login calls login and saves the returned cookie
func (m *BoschApiClient) Login() error {
	resp, err := m.Client.Get(m.uri)

	if err != nil {
		log.Fatalln(err)
		return err
	}

	if resp.StatusCode >= 300 {
		log.Fatal("Error while getting WUI SID. Response code was >=300:")
		return errors.New("Error while getting WUI SID. Response code was >=300:")
	}

	defer resp.Body.Close()

	//We Read the response body on the line below.
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
		return err
	}

	err = extractWuiSidFromBody(m, string(body))

	if err != nil {
		log.Fatal(err)
		return err
	}

	return nil
}

func readLoop(m *BoschApiClient) {
	for {
		loopError := executeRead(m)

		if loopError != nil {
			break
		}

		time.Sleep(5000 * time.Millisecond)
	}
}

func executeRead(m *BoschApiClient) error {
	var postMessge = []byte(`action=get.hyb.overview&flow=1`)
	resp, err := m.Client.Post(m.uri+"/cgi-bin/ipcclient.fcgi?"+m.wuSid, "text/plain", bytes.NewBuffer(postMessge))

	if err != nil {
		log.Fatal("error posting data retrieval request")
		return err
	}

	defer resp.Body.Close()

	//Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
		return err
	}

	if resp.StatusCode >= 300 {
		log.Fatal("Error while reading values. Response code was >=300:")
		return errors.New("Error while reading values. Response code was >=300")
	}

	sb := string(body)
	return extractValues(m, sb)
}

func parseWattValue(inputString string) (float64, error) {
	if len(strings.TrimSpace(inputString)) == 0 || strings.Contains(inputString, "nbsp;") {
		return 0.0, nil
	}

	zahlenString := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(inputString, "kW", " "), "von", " "))

	resultFloat, err := strconv.ParseFloat(zahlenString, 64)

	return resultFloat * 1000.0, err
}

func extractValues(m *BoschApiClient, body string) error {
	if strings.Contains(body, "session invalid") {
		m.Login()
		return nil
	}

	values := strings.Split(body, "|")

	soc, err := strconv.Atoi(values[3])

	if err != nil {
		return err
	}

	m.currentBatterySocValue = float64(soc)
	m.einspeisung, err = parseWattValue(values[11])

	if err != nil {
		return err
	}

	m.strombezugAusNetz, err = parseWattValue(values[14])

	if err != nil {
		return err
	}

	m.pvLeistungWatt, err = parseWattValue(values[2])

	if err != nil {
		return err
	}

	m.batterieLadeStrom, err = parseWattValue(values[10])

	if err != nil {
		return err
	}

	m.verbrauchVonBatterie, err = parseWattValue(values[13])

	return err
}

func extractWuiSidFromBody(m *BoschApiClient, body string) error {
	index := strings.Index(body, "WUI_SID=")

	if index < 0 {
		m.wuSid = ""
		return errors.New("Error while extracting WUI_SID. Body was= " + body)
	}

	m.wuSid = body[index+9 : index+9+15]

	return nil
}

// CurrentPower implements the api.Meter interface
func (m *Bosch) CurrentPower() (float64, error) {
	if m.usage == "grid" {
		if m.requestClient.einspeisung > 0.0 {
			return -1.0 * m.requestClient.einspeisung, nil
		} else {
			return m.requestClient.strombezugAusNetz, nil
		}
	}
	if m.usage == "pv" {
		return -1.0 * m.requestClient.pvLeistungWatt, nil
	}
	if m.usage == "battery" {
		if m.requestClient.batterieLadeStrom > 0.0 {
			return -1.0 * m.requestClient.batterieLadeStrom, nil
		} else {
			return m.requestClient.verbrauchVonBatterie, nil
		}
	}
	return 0.0, nil
}

// totalEnergy implements the api.MeterEnergy interface
func (m *Bosch) totalEnergy() (float64, error) {
	return m.currentTotalEnergyValue, nil
}

// batterySoC implements the api.Battery interface
func (m *Bosch) batterySoC() (float64, error) {
	return m.requestClient.currentBatterySocValue, nil
}
