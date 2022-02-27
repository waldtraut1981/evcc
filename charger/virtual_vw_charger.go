package charger

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/util"
	"github.com/evcc-io/evcc/util/request"
	"github.com/evcc-io/evcc/vehicle/vw"
	"github.com/thoas/go-funk"
)

const (
	expiry   = 2 * time.Minute // maximum response age before refresh
	interval = 2 * time.Minute // refresh interval when charging
)

type ChargeTransition string

// Charge modes
const (
	TransitionToEnabled  ChargeTransition = "transitionToEnabled"
	TransitionToDisabled ChargeTransition = "transitionToDisabled"
	StateReached         ChargeTransition = "stateReached"
)

type VwVirtualCharger struct {
	*vw.Provider // provides the api implementations
	ChargeTransition
	*util.Logger
	callCounterDuringTransistion int
	Latitude, Longitude          float64
}

func init() {
	registry.Add("virtualvw", NewVirtualVwFromConfig)
}

// NewVirtualVwFromConfig creates a virtual VW charger from generic config
func NewVirtualVwFromConfig(other map[string]interface{}) (api.Charger, error) {
	fmt.Println("NewVirtualVwFromConfig")
	cc := struct {
		User, Password, VIN string
		Latitude, Longitude float64
		Cache               time.Duration
		Timeout             time.Duration
	}{
		Cache:   interval,
		Timeout: request.Timeout,
	}

	if err := util.DecodeOther(other, &cc); err != nil {
		return nil, err
	}

	log := util.NewLogger("vw_virtual_charger").Redact(cc.User, cc.Password, cc.VIN)

	identity := vw.GetIdentityFromMap(cc.User)

	var err error

	if identity == nil {
		log.DEBUG.Println("no identity present for username. Creating new one.")

		identity = vw.NewIdentity(log, vw.AuthClientID, vw.AuthParams, cc.User, cc.Password)
		err := identity.Login()
		if err != nil {
			return nil, fmt.Errorf("login failed: %w", err)
		}
	} else {
		log.DEBUG.Println("Reusing identity for username.")
	}

	api := vw.GetApiFromMap(identity)

	if api == nil {
		log.DEBUG.Println("no API present for identity. Creating new one.")

		api = vw.NewAPI(log, identity, vw.Brand, vw.Country)
		api.Client.Timeout = cc.Timeout

		vw.AddApiToMap(identity, api)
	} else {
		log.DEBUG.Println("Reusing API for identity.")
	}

	cc.VIN, err = ensureVehicle(cc.VIN, api.Vehicles)

	provider := vw.GetProviderFromMap(cc.VIN)

	if provider == nil {
		log.DEBUG.Println("no provider for VIN. Creating new one.")

		if err == nil {
			if err = api.HomeRegion(cc.VIN); err == nil {
				provider = vw.NewProvider(api, cc.VIN, cc.Cache)
			}
		}
	} else {
		log.DEBUG.Println("Reusing provider for VIN.")
	}

	return NewVwVirtual(provider, log, cc.Latitude, cc.Longitude)
}

// NewVwVirtual creates VW virtual charger
func NewVwVirtual(provider *vw.Provider, log *util.Logger, latitude float64, longitude float64) (*VwVirtualCharger, error) {
	c := &VwVirtualCharger{
		Provider:                     provider,
		ChargeTransition:             StateReached,
		Logger:                       log,
		callCounterDuringTransistion: 0,
		Latitude:                     latitude,
		Longitude:                    longitude,
	}

	return c, nil
}

// Enabled implements the api.Charger interface
func (c *VwVirtualCharger) Enabled() (bool, error) {

	status, err := c.Status()

	if err == nil {
		isCharging := (status == api.StatusC) || (status == api.StatusD)
		return isCharging, nil
	} else {
		c.Logger.ERROR.Println("Error getting status: ", err)
		return false, err
	}
}

// Enable implements the api.Charger interface
func (c *VwVirtualCharger) Enable(enable bool) error {
	if enable {
		if c.ChargeTransition == StateReached {
			c.Logger.DEBUG.Println("start charge")
			c.callCounterDuringTransistion = 0

			c.ChargeTransition = TransitionToEnabled

			return c.Provider.StartCharge()
		} else {
			c.Logger.DEBUG.Println("still in transition state to starting. doing nothing. Call nr. ", c.callCounterDuringTransistion)

			c.callCounterDuringTransistion++

			if c.callCounterDuringTransistion >= 10 {
				c.callCounterDuringTransistion = 0

				c.Logger.DEBUG.Println("10 tries during waiting. Sending start call again")

				return c.Provider.StartCharge()
			} else {
				return nil
			}
		}
	} else {
		if c.ChargeTransition == StateReached {
			c.Logger.DEBUG.Println("stop charge")
			c.callCounterDuringTransistion = 0

			c.ChargeTransition = TransitionToDisabled
			return c.Provider.StopCharge()
		} else {
			c.Logger.DEBUG.Println("still in transition state to stopped. doing nothing.")

			c.callCounterDuringTransistion++

			if c.callCounterDuringTransistion >= 10 {
				c.callCounterDuringTransistion = 0

				c.Logger.DEBUG.Println("10 tries during waiting. Sending stop call again")

				return c.Provider.StopCharge()
			} else {
				return nil
			}
		}
	}
}

// MaxCurrent implements the api.Charger interface
func (c *VwVirtualCharger) MaxCurrent(current int64) error {
	c.Logger.DEBUG.Println("set max current to %w", current)
	return nil
}

// Status implements the api.Charger interface
func (c *VwVirtualCharger) Status() (api.ChargeStatus, error) {
	//TODO: check car position before state --> is car at home near charger?

	latitude, longitude, err := c.Provider.Position()

	if err == nil {
		c.Logger.DEBUG.Println("Current vehicle position: lat ", latitude, " long ", longitude, " charger location: lat ", c.Latitude, " long ", c.Longitude)

		distance := distance(c.Latitude, c.Longitude, latitude, longitude, "K")
		c.Logger.DEBUG.Println("Distance from vehicle to charger is (km): ", distance)

		if distance > 2.0 {
			c.Logger.DEBUG.Println("vehicle is not considred to be home. Setting to fixed status StatusA (not connected)")
			return api.StatusA, nil
		}
	} else {
		c.Logger.ERROR.Println("Error getting vehicle position: ", err)
	}

	chargeStatus, err := c.Provider.Status()

	switch chargeStatus {
	// Fzg. angeschlossen: nein    Laden aktiv: nein    - Kabel nicht angeschlossen
	case api.StatusA:
		{
			c.ChargeTransition = StateReached
			break
		}
	// Fzg. angeschlossen:   ja    Laden aktiv: nein    - Kabel angeschlossen
	case api.StatusB:
		{
			if c.ChargeTransition == TransitionToDisabled {
				c.ChargeTransition = StateReached
			}
			break
		}
	// Fzg. angeschlossen:   ja    Laden aktiv:   ja    - Laden
	case api.StatusC:
	// Fzg. angeschlossen:   ja    Laden aktiv:   ja    - Laden mit Lüfter
	case api.StatusD:
		{
			if c.ChargeTransition == TransitionToEnabled {
				c.ChargeTransition = StateReached
			}
			break
		}
	// Fzg. angeschlossen:   ja    Laden aktiv: nein    - Fehler (Kurzschluss)
	case api.StatusE:
	// Fzg. angeschlossen:   ja    Laden aktiv: nein    - Fehler (Ausfall Wallbox)
	case api.StatusF:
		{
			c.ChargeTransition = StateReached
			break
		}
	default:
		{
			c.ChargeTransition = StateReached
			break
		}
	}

	if err != nil {
		c.Logger.ERROR.Println("Error getting status", err)
	}

	return chargeStatus, err
}

// findVehicle finds the first vehicle in the list of VINs or returns an error
func findVehicle(vehicles []string, err error) (string, error) {
	if err != nil {
		return "", fmt.Errorf("cannot get vehicles: %w", err)
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

// ensureVehicle ensures that the vehicle is available on the api and returns the VIN
func ensureVehicle(vin string, fun func() ([]string, error)) (string, error) {
	vehicles, err := fun()
	if err != nil {
		return "", fmt.Errorf("cannot get vehicles: %w", err)
	}

	if vin = strings.ToUpper(vin); vin != "" {
		// vin defined but doesn't exist
		if !funk.ContainsString(vehicles, vin) {
			err = fmt.Errorf("cannot find vehicle: %s", vin)
		}
	} else {
		// vin empty
		vin, err = findVehicle(vehicles, nil)
	}

	return vin, err
}

//:::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::
//:::                                                                         :::
//:::  This routine calculates the distance between two points (given the     :::
//:::  latitude/longitude of those points). It is being used to calculate     :::
//:::  the distance between two locations using GeoDataSource (TM) products  :::
//:::                                                                         :::
//:::  Definitions:                                                           :::
//:::    South latitudes are negative, east longitudes are positive           :::
//:::                                                                         :::
//:::  Passed to function:                                                    :::
//:::    lat1, lon1 = Latitude and Longitude of point 1 (in decimal degrees)  :::
//:::    lat2, lon2 = Latitude and Longitude of point 2 (in decimal degrees)  :::
//:::    unit = the unit you desire for results                               :::
//:::           where: 'M' is statute miles (default)                         :::
//:::                  'K' is kilometers                                      :::
//:::                  'N' is nautical miles                                  :::
//:::                                                                         :::
//:::  Worldwide cities and other features databases with latitude longitude  :::
//:::  are available at https://www.geodatasource.com                         :::
//:::                                                                         :::
//:::  For enquiries, please contact sales@geodatasource.com                  :::
//:::                                                                         :::
//:::  Official Web site: https://www.geodatasource.com                       :::
//:::                                                                         :::
//:::               GeoDataSource.com (C) All Rights Reserved 2022            :::
//:::                                                                         :::
//:::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::

func distance(lat1 float64, lng1 float64, lat2 float64, lng2 float64, unit ...string) float64 {
	const PI float64 = 3.141592653589793

	radlat1 := float64(PI * lat1 / 180)
	radlat2 := float64(PI * lat2 / 180)

	theta := float64(lng1 - lng2)
	radtheta := float64(PI * theta / 180)

	dist := math.Sin(radlat1)*math.Sin(radlat2) + math.Cos(radlat1)*math.Cos(radlat2)*math.Cos(radtheta)

	if dist > 1 {
		dist = 1
	}

	dist = math.Acos(dist)
	dist = dist * 180 / PI
	dist = dist * 60 * 1.1515

	if len(unit) > 0 {
		if unit[0] == "K" {
			dist = dist * 1.609344
		} else if unit[0] == "N" {
			dist = dist * 0.8684
		}
	}

	return dist
}
