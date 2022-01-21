package skoda

import (
	"fmt"
	"net/http"

	"github.com/evcc-io/evcc/util"
	"github.com/evcc-io/evcc/util/request"
	"golang.org/x/oauth2"
)

const BaseURI = "https://api.connect.skoda-auto.cz/api"

// API is the Skoda api client
type API struct {
	*request.Helper
}

// NewAPI creates a new api client
func NewAPI(log *util.Logger, ts oauth2.TokenSource) *API {
	v := &API{
		Helper: request.NewHelper(log),
	}

	v.Client.Transport = &oauth2.Transport{
		Source: ts,
		Base:   v.Client.Transport,
	}

	return v
}

// Vehicles implements the /vehicles response
func (v *API) Vehicles() ([]string, error) {
	var res VehiclesResponse

	uri := fmt.Sprintf("%s/v2/garage/vehicles", BaseURI)
	err := v.GetJSON(uri, &res)

	var vehicles []string
	if err == nil {
		for _, v := range res {
			vehicles = append(vehicles, v.VIN)
		}
	}

	return vehicles, err
}

// Charger implements the /v1/charging/<vin>/status response
func (v *API) Charger(vin string) (ChargerResponse, error) {
	var res ChargerResponse
	uri := fmt.Sprintf("%s/v1/charging/%s/status", BaseURI, vin)
	err := v.GetJSON(uri, &res)
	return res, err
}

const (
	ActionCharge      = "charging"
	ActionChargeStart = "Start"
	ActionChargeStop  = "Stop"
)

// Action executes a vehicle action
func (v *API) Action(vin, action, value string) error {
	var res map[string]interface{}
	uri := fmt.Sprintf("%s/v1/%s/operation-requests?vin=%s", BaseURI, action, vin)

	data := struct {
		Typ string `json:"type"`
	}{
		Typ: value,
	}

	req, err := request.New(http.MethodPost, uri, request.MarshalJSON(data), request.JSONEncoding)
	if err == nil {
		// {"id":"61991908906fa40af9a5cba4","status":"InProgress","deeplink":""}
		err = v.DoJSON(req, &res)
	}

	return err
}
