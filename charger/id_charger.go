package charger

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/charger/idcharger"
	"github.com/evcc-io/evcc/util"
	"github.com/evcc-io/evcc/util/request"
	"github.com/evcc-io/evcc/vehicle/vw"
)

const (
	interval = 2 * time.Minute // refresh interval when charging
)

// IdCharger charger implementation
type IdCharger struct {
	log                 *util.Logger
	current             uint16
	*idcharger.Provider // provides the api implementations
}

func init() {
	registry.Add("idcharger", NewIdChargerFromConfig)
}

// NewIdChargerFromConfig creates a ID.Charger (virtual) charger from generic config
func NewIdChargerFromConfig(other map[string]interface{}) (api.Charger, error) {
	cc := struct {
		User, Password, VIN string
		Cache               time.Duration
		Timeout             time.Duration
	}{
		Cache:   interval,
		Timeout: request.Timeout,
	}

	if err := util.DecodeOther(other, &cc); err != nil {
		return nil, err
	}

	return NewIdCharger(cc.User, cc.Password, cc.VIN, cc.Timeout, cc.Cache)
}

// NewIdCharger creates ID.Charger (virtual) charger
func NewIdCharger(user, password, vin string, timeout, cache time.Duration) (api.Charger, error) {
	log := util.NewLogger("idcharger")

	wb := &IdCharger{
		log:     log,
		current: 60, // assume min current
	}

	identity := vw.NewIdentity(log)

	query := url.Values(map[string][]string{
		"response_type": {"code id_token token"},
		"client_id":     {"a24fba63-34b3-4d43-b181-942111e6bda8@apps_vw-dilab_com"},
		"redirect_uri":  {"weconnect://authenticated"},
		"scope":         {"openid profile badge cars dealers vin"},
	})

	err := identity.LoginID(query, user, password)
	if err != nil {
		return wb, fmt.Errorf("login failed: %w", err)
	}

	api := idcharger.NewAPI(log, identity)

	api.Client.Timeout = timeout

	if vin == "" {
		vin, err = findVehicle(api.Vehicles())
		if err == nil {
			log.DEBUG.Printf("found vehicle: %v", vin)
		}
	}

	wb.Provider = idcharger.NewProvider(api, strings.ToUpper(vin), cache)

	return wb, nil
}

func findVehicle(vehicles []string, err error) (string, error) {
	if err != nil {
		return "", fmt.Errorf("cannot get vehicles: %v", err)
	}

	if len(vehicles) != 1 {
		return "", fmt.Errorf("cannot find vehicle: %v", vehicles)
	}

	vin := strings.TrimSpace(vehicles[0])
	if vin == "" {
		return "", fmt.Errorf("cannot find vehicle: %v", vehicles)
	}

	return vin, nil
}

// Status implements the api.Charger interface
func (wb *IdCharger) Status() (api.ChargeStatus, error) {
	status, err := wb.Provider.Status()
	return status, err
}

// Enabled implements the api.Charger interface
func (wb *IdCharger) Enabled() (bool, error) {
	chargerStatus, err := wb.Provider.Status()

	if err != nil {
		return false, err
	}

	if chargerStatus == api.StatusC {
		return true, err
	} else {
		return false, err
	}
}

// Enable implements the api.Charger interface
func (wb *IdCharger) Enable(enable bool) error {
	if enable {
		return wb.Provider.StartCharge()
	} else {
		return wb.Provider.StopCharge()
	}
}

// MaxCurrent implements the api.Charger interface
func (wb *IdCharger) MaxCurrent(current int64) error {
	return nil
}

var _ api.ChargerEx = (*IdCharger)(nil)

// MaxCurrentMillis implements the api.ChargerEx interface
func (wb *IdCharger) MaxCurrentMillis(current float64) error {
	return nil
}

var _ api.Meter = (*IdCharger)(nil)

// CurrentPower implements the api.Meter interface
func (wb *IdCharger) CurrentPower() (float64, error) {
	return float64(0), nil
}

var _ api.MeterEnergy = (*IdCharger)(nil)

// TotalEnergy implements the api.MeterEnergy interface
func (wb *IdCharger) TotalEnergy() (float64, error) {
	return float64(0) / 1e3, nil
}

var _ api.MeterCurrent = (*IdCharger)(nil)

// Currents implements the api.MeterCurrent interface
func (wb *IdCharger) Currents() (float64, float64, float64, error) {
	var currents []float64
	for range hecRegCurrents {
		currents = append(currents, float64(0)/10)
	}

	return currents[0], currents[1], currents[2], nil
}

var _ api.Diagnosis = (*IdCharger)(nil)

// Diagnose implements the api.Diagnosis interface
func (wb *IdCharger) Diagnose() {
	fmt.Printf("Test")
}
