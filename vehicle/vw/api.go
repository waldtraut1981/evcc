package vw

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/evcc-io/evcc/util"
	"github.com/evcc-io/evcc/util/request"
	"golang.org/x/oauth2"
)

// DefaultBaseURI is the VW api base URI
const DefaultBaseURI = "https://msg.volkswagen.de/fs-car"

// RegionAPI is the VW api used for determining the home region
const RegionAPI = "https://mal-1a.prd.ece.vwg-connect.com/api"

// API is the VW api client
type API struct {
	*request.Helper
	brand, country string
	baseURI        string
	statusURI      string
	logger         *util.Logger
}

var apiInstances map[*Identity]*API = make(map[*Identity]*API)

func GetApiFromMap(ident *Identity) *API {
	return apiInstances[ident]
}

func AddApiToMap(ident *Identity, api *API) {
	apiInstances[ident] = api
}

// NewAPI creates a new api client
func NewAPI(log *util.Logger, ts oauth2.TokenSource, brand, country string) *API {
	v := &API{
		Helper:  request.NewHelper(log),
		brand:   brand,
		country: country,
		baseURI: DefaultBaseURI,
		logger:  log,
	}

	v.Client.Transport = &oauth2.Transport{
		Source: ts,
		Base:   v.Client.Transport,
	}

	return v
}

// Vehicles implements the /vehicles response
func (v *API) Vehicles() ([]string, error) {
	v.logger.DEBUG.Println("API_CALL: place call Vehicles")
	var res VehiclesResponse
	uri := fmt.Sprintf("%s/usermanagement/users/v1/%s/%s/vehicles", v.baseURI, v.brand, v.country)
	err := v.GetJSON(uri, &res)
	if err != nil && res.Error != nil {
		err = res.Error.Error()
		v.logger.DEBUG.Println("API_CALL: error during Vehicles call: ", err)
	} else {
		v.logger.DEBUG.Println("API_CALL: Vehicles answer: ", res.UserVehicles.Vehicle)
	}
	return res.UserVehicles.Vehicle, err
}

// HomeRegion updates the home region for the given vehicle
func (v *API) HomeRegion(vin string) error {
	v.logger.DEBUG.Println("API_CALL: place call HomeRegion")
	var res HomeRegion
	uri := fmt.Sprintf("%s/cs/vds/v1/vehicles/%s/homeRegion", RegionAPI, vin)

	err := v.GetJSON(uri, &res)
	if err == nil {
		v.logger.DEBUG.Println("API_CALL: HomeRegion answer: ", res.HomeRegion)
		if api := res.HomeRegion.BaseURI.Content; strings.HasPrefix(api, "https://mal-3a.prd.eu.dp.vwg-connect.com") {
			api = "https://fal" + strings.TrimPrefix(api, "https://mal")
			api = strings.TrimSuffix(api, "/api") + "/fs-car"
			v.baseURI = api
		}
	} else if res.Error != nil {
		err = res.Error.Error()
		v.logger.DEBUG.Println("API_CALL: error during HomeRegion call: ", err)
	}

	return err
}

// RolesRights implements the /rolesrights/operationlist response
func (v *API) RolesRights(vin string) (RolesRights, error) {
	v.logger.DEBUG.Println("API_CALL: place call RolesRights")
	var res RolesRights
	uri := fmt.Sprintf("%s/rolesrights/operationlist/v3/vehicles/%s", RegionAPI, vin)
	err := v.GetJSON(uri, &res)
	v.logger.DEBUG.Println("API_CALL: RolesRights result:", err, res)
	return res, err
}

// ServiceURI renders the service URI for the given vin and service
func (v *API) ServiceURI(vin, service string, rr RolesRights) (uri string) {
	if si := rr.ServiceByID(service); si != nil {
		uri = si.InvocationUrl.Content
		uri = strings.ReplaceAll(uri, "{vin}", vin)
		uri = strings.ReplaceAll(uri, "{brand}", v.brand)
		uri = strings.ReplaceAll(uri, "{country}", v.country)
	}

	return uri
}

// Status implements the /status response
func (v *API) Status(vin string) (StatusResponse, error) {
	v.logger.DEBUG.Println("API_CALL: place call Status")
	var res StatusResponse
	uri := fmt.Sprintf("%s/bs/vsr/v1/vehicles/%s/status", RegionAPI, vin)
	if v.statusURI != "" {
		uri = v.statusURI
	}

	headers := map[string]string{
		"Accept":        request.JSONContent,
		"X-App-Name":    "foo", // required
		"X-App-Version": "foo", // required
	}

	req, err := request.New(http.MethodGet, uri, nil, headers)
	if err == nil {
		err = v.DoJSON(req, &res)
		v.logger.DEBUG.Println("API_CALL: result get Status", res)
	} else {
		v.logger.DEBUG.Println("API_CALL: error during get Status", err)
	}

	if _, ok := err.(request.StatusError); ok {
		var rr RolesRights
		rr, err = v.RolesRights(vin)

		if err == nil {
			if uri = v.ServiceURI(vin, StatusService, rr); uri == "" {
				err = fmt.Errorf("%s not found", StatusService)
			}
		}

		if err == nil {
			if strings.HasSuffix(uri, fmt.Sprintf("%s/", vin)) {
				uri += "status"
			}

			v.logger.DEBUG.Println("API_CALL: place call Status step 2")
			if req, err = request.New(http.MethodGet, uri, nil, headers); err == nil {
				if err = v.DoJSON(req, &res); err == nil {
					v.logger.DEBUG.Println("API_CALL: result get Status step 2:", res)
					v.statusURI = uri
				} else {
					v.logger.DEBUG.Println("API_CALL: error during get Status step 2 ", err)
				}
			}
		}
	}

	return res, err
}

// Charger implements the /charger response
func (v *API) Charger(vin string) (ChargerResponse, error) {
	v.logger.DEBUG.Println("API_CALL: place call Charger")
	var res ChargerResponse
	uri := fmt.Sprintf("%s/bs/batterycharge/v1/%s/%s/vehicles/%s/charger", v.baseURI, v.brand, v.country, vin)
	err := v.GetJSON(uri, &res)
	if err != nil && res.Error != nil {
		err = res.Error.Error()
		v.logger.DEBUG.Println("API_CALL: error call Charger ", err)
	} else {
		v.logger.DEBUG.Println("API_CALL: call Charger result ", res)
	}
	return res, err
}

// Climater implements the /climater response
func (v *API) Climater(vin string) (ClimaterResponse, error) {
	v.logger.DEBUG.Println("API_CALL: place call Climater")
	var res ClimaterResponse
	uri := fmt.Sprintf("%s/bs/climatisation/v1/%s/%s/vehicles/%s/climater", v.baseURI, v.brand, v.country, vin)
	err := v.GetJSON(uri, &res)
	if err != nil && res.Error != nil {
		err = res.Error.Error()
		v.logger.DEBUG.Println("API_CALL: error call Climater ", err)
	} else {
		v.logger.DEBUG.Println("API_CALL: call Climater result ", res)
	}
	return res, err
}

// Position implements the /position response
func (v *API) Position(vin string) (PositionResponse, error) {
	v.logger.DEBUG.Println("API_CALL: place call Position")
	var res PositionResponse
	uri := fmt.Sprintf("%s/bs/cf/v1/%s/%s/vehicles/%s/position", v.baseURI, v.brand, v.country, vin)

	req, err := request.New(http.MethodGet, uri, nil, map[string]string{
		"Accept":        request.JSONContent,
		"Content-type":  "application/vnd.vwg.mbb.carfinderservice_v1_0_0+json",
		"X-App-Version": "TODO",
		"X-App-Name":    "TODO",
	})
	if err == nil {
		err = v.DoJSON(req, &res)
	}

	if err != nil && res.Error != nil {
		err = res.Error.Error()
		v.logger.DEBUG.Println("API_CALL: error call Position ", err)
	} else {
		v.logger.DEBUG.Println("API_CALL: call Position result ", res)
	}

	return res, err
}

const (
	ActionCharge      = "batterycharge"
	ActionChargeStart = "start"
	ActionChargeStop  = "stop"
)

type actionDefinition struct {
	contentType string
	appendix    string
}

var actionDefinitions = map[string]actionDefinition{
	ActionCharge: {
		"application/vnd.vwg.mbb.ChargerAction_v1_0_0+xml",
		"charger/actions",
	},
}

// Action implements vehicle actions
func (v *API) Action(vin, action, value string) error {
	v.logger.DEBUG.Println("API_CALL: place call Action")
	def := actionDefinitions[action]

	uri := fmt.Sprintf("%s/bs/%s/v1/%s/%s/vehicles/%s/%s", v.baseURI, action, v.brand, v.country, vin, def.appendix)
	body := "<?xml version=\"1.0\" encoding=\"UTF-8\" ?><action><type>" + value + "</type></action>"

	req, err := request.New(http.MethodPost, uri, strings.NewReader(body), map[string]string{
		"Content-type": def.contentType,
	})

	if err == nil {
		var resp *http.Response
		if resp, err = v.Do(req); err == nil {
			resp.Body.Close()
			v.logger.DEBUG.Println("API_CALL: call Action result ", resp.Body)
		} else {
			v.logger.DEBUG.Println("API_CALL: error during call Action ", err)
		}
	} else {
		v.logger.DEBUG.Println("API_CALL: error during call Action ", err)
	}

	return err
}

// Any implements any api response
func (v *API) Any(base, vin string) (interface{}, error) {
	var res interface{}
	uri := fmt.Sprintf("%s/"+strings.TrimLeft(base, "/"), v.baseURI, v.brand, v.country, vin)
	err := v.GetJSON(uri, &res)
	return res, err
}
