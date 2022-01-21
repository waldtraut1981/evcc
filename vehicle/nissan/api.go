package nissan

import (
	"fmt"
	"net/http"
	"time"

	"github.com/evcc-io/evcc/util"
	"github.com/evcc-io/evcc/util/request"
	"golang.org/x/oauth2"
)

// api constants
const (
	APIVersion         = "protocol=1.0,resource=2.1"
	ClientID           = "a-ncb-prod-android"
	ClientSecret       = "3LBs0yOx2XO-3m4mMRW27rKeJzskhfWF0A8KUtnim8i/qYQPl8ZItp3IaqJXaYj_"
	Scope              = "openid profile vehicles"
	AuthBaseURL        = "https://prod.eu.auth.kamereon.org/kauth"
	Realm              = "a-ncb-prod"
	RedirectURI        = "org.kamereon.service.nci:/oauth2redirect"
	CarAdapterBaseURL  = "https://alliance-platform-caradapter-prod.apps.eu.kamereon.io/car-adapter"
	UserAdapterBaseURL = "https://alliance-platform-usersadapter-prod.apps.eu.kamereon.io/user-adapter"
	UserBaseURL        = "https://nci-bff-web-prod.apps.eu.kamereon.io/bff-web"
)

type API struct {
	*request.Helper
}

func NewAPI(log *util.Logger, identity oauth2.TokenSource) *API {
	v := &API{
		Helper: request.NewHelper(log),
	}

	// api is unbelievably slow when retrieving status
	v.Client.Timeout = 120 * time.Second

	// replace client transport with authenticated transport
	v.Client.Transport = &oauth2.Transport{
		Source: identity,
		Base:   v.Client.Transport,
	}

	return v
}

func (v *API) Vehicles() ([]string, error) {
	var user struct{ UserID string }
	uri := fmt.Sprintf("%s/v1/users/current", UserAdapterBaseURL)
	err := v.GetJSON(uri, &user)

	var res Vehicles
	if err == nil {
		uri := fmt.Sprintf("%s/v4/users/%s/cars", UserBaseURL, user.UserID)
		err = v.GetJSON(uri, &res)
	}

	var vehicles []string
	if err == nil {
		for _, v := range res.Data {
			vehicles = append(vehicles, v.VIN)
		}
	}

	return vehicles, err
}

// Battery provides battery api response
func (v *API) BatteryStatus(vin string) (StatusResponse, error) {
	uri := fmt.Sprintf("%s/v1/cars/%s/battery-status", CarAdapterBaseURL, vin)

	var res StatusResponse
	err := v.GetJSON(uri, &res)

	return res, err
}

// RefreshRequest requests  battery status refresh
func (v *API) RefreshRequest(vin string, typ string) (ActionResponse, error) {
	var res ActionResponse
	uri := fmt.Sprintf("%s/v1/cars/%s/actions/refresh-battery-status", CarAdapterBaseURL, vin)

	data := Request{
		Data: Payload{
			Type: typ,
		},
	}

	req, err := request.New(http.MethodPost, uri, request.MarshalJSON(data), map[string]string{
		"Content-Type": "application/vnd.api+json",
	})

	if err == nil {
		err = v.DoJSON(req, &res)
	}

	return res, err
}

type Action string

const (
	ActionChargeStart Action = "start"
	ActionChargeStop  Action = "stop"
)

// ChargingAction provides actions/charging-start api response
func (v *API) ChargingAction(vin string, action Action) (ActionResponse, error) {
	uri := fmt.Sprintf("%s/v1/cars/%s/actions/charging-start", CarAdapterBaseURL, vin)

	data := Request{
		Data: Payload{
			Type: "ChargingStart",
			Attributes: map[string]interface{}{
				"action": action,
			},
		},
	}

	req, err := request.New(http.MethodPost, uri, request.MarshalJSON(data), map[string]string{
		"Content-Type": "application/vnd.api+json",
	})

	var res ActionResponse
	if err == nil {
		err = v.DoJSON(req, &res)
	}

	return res, err
}
