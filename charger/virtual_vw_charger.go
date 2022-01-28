package charger

import (
	"fmt"
	"strings"
	"time"

	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/util"
	"github.com/evcc-io/evcc/util/request"
	"github.com/evcc-io/evcc/vehicle/vw"
	"github.com/thoas/go-funk"
)

const (
	expiry   = 5 * time.Minute  // maximum response age before refresh
	interval = 15 * time.Minute // refresh interval when charging
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
}

func init() {
	registry.Add("virtualvw", NewVirtualVwFromConfig)
}

// NewVirtualVwFromConfig creates a virtual VW charger from generic config
func NewVirtualVwFromConfig(other map[string]interface{}) (api.Charger, error) {
	fmt.Println("NewVirtualVwFromConfig")
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

	return NewVwVirtual(provider, log)
}

// NewVwVirtual creates VW virtual charger
func NewVwVirtual(provider *vw.Provider, log *util.Logger) (*VwVirtualCharger, error) {
	c := &VwVirtualCharger{
		Provider:         provider,
		ChargeTransition: StateReached,
		Logger:           log,
	}

	return c, nil
}

// Enabled implements the api.Charger interface
func (c *VwVirtualCharger) Enabled() (bool, error) {

	status, err := c.Provider.Status()

	if err == nil {
		return status == api.StatusC, nil
	}

	return false, nil
}

// Enable implements the api.Charger interface
func (c *VwVirtualCharger) Enable(enable bool) error {
	if enable {
		if c.ChargeTransition == StateReached {
			c.Logger.DEBUG.Println("start charge")

			c.ChargeTransition = TransitionToEnabled
			return c.Provider.StartCharge()
		} else {
			c.Logger.DEBUG.Println("still in transition state to starting. doing nothing.")

			return nil
		}
	} else {
		if c.ChargeTransition == StateReached {
			c.Logger.DEBUG.Println("stop charge")

			c.ChargeTransition = TransitionToDisabled
			return c.Provider.StartCharge()
		} else {
			c.Logger.DEBUG.Println("still in transition state to stopped. doing nothing.")

			return nil
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
	chargeStatus, err := c.Provider.Status()

	if c.ChargeTransition == TransitionToEnabled && chargeStatus == api.StatusC {
		c.Logger.DEBUG.Println("state enabled reached in API call")
		c.ChargeTransition = StateReached
	}

	if c.ChargeTransition == TransitionToDisabled && (chargeStatus == api.StatusB || chargeStatus == api.StatusA) {
		c.Logger.DEBUG.Println("state disabled reached in API call")
		c.ChargeTransition = StateReached
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
