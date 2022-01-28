package vehicle

import (
	"fmt"
	"time"

	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/util"
	"github.com/evcc-io/evcc/util/request"
	"github.com/evcc-io/evcc/vehicle/vw"
)

// https://github.com/trocotronic/weconnect
// https://github.com/TA2k/ioBroker.vw-connect

// VW is an api.Vehicle implementation for VW cars
type VW struct {
	*embed
	*vw.Provider // provides the api implementations
}

func init() {
	registry.Add("vw", NewVWFromConfig)
}

// NewVWFromConfig creates a new vehicle
func NewVWFromConfig(other map[string]interface{}) (api.Vehicle, error) {
	cc := struct {
		embed               `mapstructure:",squash"`
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

	v := &VW{
		embed: &cc.embed,
	}

	log := util.NewLogger("vw").Redact(cc.User, cc.Password, cc.VIN)

	identity := vw.GetIdentityFromMap(cc.User)

	var err error

	if identity == nil {
		log.ERROR.Println("no identity present for username. Creating new one.")

		identity = vw.NewIdentity(log, vw.AuthClientID, vw.AuthParams, cc.User, cc.Password)
		err := identity.Login()
		if err != nil {
			return v, fmt.Errorf("login failed: %w", err)
		}
	} else {
		log.ERROR.Println("Reusing identity for username.")
	}

	api := vw.GetApiFromMap(identity)

	if api == nil {
		log.ERROR.Println("no API present for identity. Creating new one.")

		api = vw.NewAPI(log, identity, vw.Brand, vw.Country)
		api.Client.Timeout = cc.Timeout

		vw.AddApiToMap(identity, api)
	} else {
		log.ERROR.Println("Reusing API for identity.")
	}

	cc.VIN, err = ensureVehicle(cc.VIN, api.Vehicles)

	v.Provider = vw.GetProviderFromMap(cc.VIN)

	if v.Provider == nil {
		log.ERROR.Println("no provider for VIN. Creating new one.")

		if err == nil {
			if err = api.HomeRegion(cc.VIN); err == nil {
				v.Provider = vw.NewProvider(api, cc.VIN, cc.Cache)
			}
		}
	} else {
		log.ERROR.Println("Reusing provider for VIN.")
	}

	return v, err
}
