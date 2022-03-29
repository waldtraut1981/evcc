package bosch

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http/cookiejar"
	"strconv"
	"strings"
	"time"

	"github.com/evcc-io/evcc/util"
	"github.com/evcc-io/evcc/util/request"
	"github.com/evcc-io/evcc/util/transport"
)

type API interface {
	Login() error
	SellToGrid() (float64, error)
	BuyFromGrid() (float64, error)
	PvPower() (float64, error)
	BatteryChargePower() (float64, error)
	BatteryDischargePower() (float64, error)
	BatterySoc() (float64, error)
}

type LocalAPI struct {
	*request.Helper
	uri     string
	status  StatusResponse
	login   LoginResponse
	updated time.Time
	cache   time.Duration
	logger  *util.Logger
}

var _ API = (*LocalAPI)(nil)

func NewLocal(log *util.Logger, uri string, cache time.Duration) *LocalAPI {

	initialStatus := StatusResponse{
		currentBatterySoc:     0.0,
		sellToGrid:            0.0,
		buyFromGrid:           0.0,
		pvPower:               0.0,
		batteryChargePower:    0.0,
		batteryDischargePower: 0.0,
	}

	api := &LocalAPI{
		Helper: request.NewHelper(log),
		uri:    util.DefaultScheme(strings.TrimSuffix(uri, "/"), "http"),
		cache:  cache,
		logger: log,
		status: initialStatus,
	}

	// ignore the self signed certificate
	api.Client.Transport = request.NewTripper(log, transport.Insecure())
	// create cookie jar to save login tokens
	api.Client.Jar, _ = cookiejar.New(nil)

	return api
}

func (c *LocalAPI) Login() (err error) {
	resp, err := c.Client.Get(c.uri)

	if err != nil {
		return fmt.Errorf("error during login: first get: %s", err)
	}

	if resp.StatusCode >= 300 {
		return errors.New("error while getting wui sid. response code was >=300")
	}

	defer resp.Body.Close()

	//We Read the response body on the line below.
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error during login: read response body: %s", err)
	}

	err = extractWuiSidFromBody(c, string(body))

	if err != nil {
		return fmt.Errorf("error during login: error extract wui sid: %s", err)
	}

	return nil
}

//////////// value retrieval ////////////////

func (c *LocalAPI) SellToGrid() (res float64, err error) {
	if time.Since(c.updated) > c.cache {

		err = c.updateValues()

		if err == nil {
			c.updated = time.Now()
		}
	}
	return c.status.sellToGrid, err
}

func (c *LocalAPI) BuyFromGrid() (res float64, err error) {
	if time.Since(c.updated) > c.cache {

		err = c.updateValues()

		if err == nil {
			c.updated = time.Now()
		}
	}
	return c.status.buyFromGrid, err
}

func (c *LocalAPI) PvPower() (res float64, err error) {
	if time.Since(c.updated) > c.cache {

		err = c.updateValues()

		if err == nil {
			c.updated = time.Now()
		}
	}
	return c.status.pvPower, err
}

func (c *LocalAPI) BatteryChargePower() (res float64, err error) {
	if time.Since(c.updated) > c.cache {

		err = c.updateValues()

		if err == nil {
			c.updated = time.Now()
		}
	}
	return c.status.batteryChargePower, err
}

func (c *LocalAPI) BatteryDischargePower() (res float64, err error) {
	if time.Since(c.updated) > c.cache {

		err = c.updateValues()

		if err == nil {
			c.updated = time.Now()
		}
	}
	return c.status.batteryDischargePower, err
}

func (c *LocalAPI) BatterySoc() (res float64, err error) {
	if time.Since(c.updated) > c.cache {

		err = c.updateValues()

		if err == nil {
			c.updated = time.Now()
		}
	}
	return c.status.currentBatterySoc, err
}

//////////// helpers ////////////////

func extractWuiSidFromBody(c *LocalAPI, body string) error {
	index := strings.Index(body, "WUI_SID=")

	if index < 0 {
		c.login.wuSid = ""
		return fmt.Errorf("error while extracting wui sid. body was= %s", body)
	}

	c.login.wuSid = body[index+9 : index+9+15]

	c.logger.DEBUG.Println("extractWuiSidFromBody: result=", c.login.wuSid)

	return nil
}

func (c *LocalAPI) updateValues() error {
	var postMessge = []byte(`action=get.hyb.overview&flow=1`)
	resp, err := c.Client.Post(c.uri+"/cgi-bin/ipcclient.fcgi?"+c.login.wuSid, "text/plain", bytes.NewBuffer(postMessge))

	if err != nil {
		return fmt.Errorf("error during data retrieval request: post: %s", err)
	}

	defer resp.Body.Close()

	//Read the response body
	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return fmt.Errorf("error during data retrieval request: read body: %s", err)
	}

	if resp.StatusCode >= 300 {
		return errors.New("error while reading values. response code was >=300")
	}

	sb := string(body)
	return extractValues(c, sb)
}

func parseWattValue(inputString string) (float64, error) {
	if len(strings.TrimSpace(inputString)) == 0 || strings.Contains(inputString, "nbsp;") {
		return 0.0, nil
	}

	zahlenString := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(inputString, "kW", " "), "von", " "))

	resultFloat, err := strconv.ParseFloat(zahlenString, 64)

	return resultFloat * 1000.0, err
}

func extractValues(c *LocalAPI, body string) error {
	if strings.Contains(body, "session invalid") {
		c.logger.DEBUG.Println("extractValues: Session invalid. Performing Re-login")
		return c.Login()
	}

	values := strings.Split(body, "|")

	soc, err := strconv.Atoi(values[3])

	if err != nil {
		return fmt.Errorf("extractValues: error during value parsing 1: %s", err)
	}

	c.status.currentBatterySoc = float64(soc)
	c.status.sellToGrid, err = parseWattValue(values[11])

	if err != nil {
		return fmt.Errorf("extractValues: error during value parsing 2: %s", err)
	}

	c.status.buyFromGrid, err = parseWattValue(values[14])

	if err != nil {
		return fmt.Errorf("extractValues: error during value parsing 3: %s", err)
	}

	c.status.pvPower, err = parseWattValue(values[2])

	if err != nil {
		return fmt.Errorf("extractValues: error during value parsing 4: %s", err)
	}

	c.status.batteryChargePower, err = parseWattValue(values[10])

	if err != nil {
		return fmt.Errorf("extractValues: error during value parsing 5: %s", err)
	}

	c.status.batteryDischargePower, err = parseWattValue(values[13])

	c.logger.DEBUG.Println("extractValues: batterieLadeStrom=", c.status.batteryChargePower, ";currentBatterySocValue=", c.status.currentBatterySoc, ";einspeisung=", c.status.sellToGrid, ";pvLeistungWatt=", c.status.pvPower, ";strombezugAusNetz=", c.status.buyFromGrid, ";verbrauchVonBatterie=", c.status.batteryDischargePower)

	return err
}
