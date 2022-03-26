package id

import (
	"strings"
	"time"

	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/provider"
)

// Provider is an api.Vehicle implementation for VW ID cars
type Provider struct {
	statusG func() (interface{}, error)
	action  func(action, value string) error
}

// NewProvider creates a new vehicle
func NewProvider(api *API, vin string, cache time.Duration) *Provider {
	impl := &Provider{
		statusG: provider.NewCached(func() (interface{}, error) {
			return api.Status(vin)
		}, cache).InterfaceGetter(),
		action: func(action, value string) error {
			return api.Action(vin, action, value)
		},
	}
	return impl
}

var _ api.Battery = (*Provider)(nil)

// SoC implements the api.Vehicle interface
func (v *Provider) SoC() (float64, error) {
	res, err := v.statusG()
	if res, ok := res.(Status); err == nil && ok {
		return float64(res.Data.BatteryStatus.CurrentSOCPercent), nil
	}

	return 0, err
}

var _ api.ChargeState = (*Provider)(nil)

// Status implements the api.ChargeState interface
func (v *Provider) Status() (api.ChargeStatus, error) {
	status := api.StatusA // disconnected

	res, err := v.statusG()
	if res, ok := res.(Status); err == nil && ok {
		if res.Data.PlugStatus.PlugConnectionState == "connected" {
			status = api.StatusB
		}
		if res.Data.ChargingStatus.ChargingState == "charging" {
			status = api.StatusC
		}
	}

	return status, err
}

var _ api.VehicleFinishTimer = (*Provider)(nil)

// FinishTime implements the api.VehicleFinishTimer interface
func (v *Provider) FinishTime() (time.Time, error) {
	res, err := v.statusG()
	if res, ok := res.(Status); err == nil && ok {
		cst := res.Data.ChargingStatus
		return cst.CarCapturedTimestamp.Add(time.Duration(cst.RemainingChargingTimeToCompleteMin) * time.Minute), err
	}

	return time.Time{}, err
}

var _ api.VehicleRange = (*Provider)(nil)

// Range implements the api.VehicleRange interface
func (v *Provider) Range() (int64, error) {
	res, err := v.statusG()
	if res, ok := res.(Status); err == nil && ok {
		return int64(res.Data.BatteryStatus.CruisingRangeElectricKm), nil
	}

	return 0, err
}

var _ api.VehicleOdometer = (*Provider)(nil)

// Odometer implements the api.VehicleOdometer interface
func (v *Provider) Odometer() (float64, error) {
	res, err := v.statusG()
	if res, ok := res.(Status); err == nil && ok {
		if res.Data.MaintenanceStatus == nil {
			return 0, api.ErrNotAvailable
		}
		return float64(res.Data.MaintenanceStatus.MileageKm), nil
	}

	return 0, err
}

var _ api.VehicleClimater = (*Provider)(nil)

// Climater implements the api.VehicleClimater interface
func (v *Provider) Climater() (active bool, outsideTemp float64, targetTemp float64, err error) {
	res, err := v.statusG()
	if res, ok := res.(Status); err == nil && ok {
		state := strings.ToLower(res.Data.ClimatisationStatus.ClimatisationState)

		if state == "" {
			return false, 0, 0, api.ErrNotAvailable
		}

		active := state != "off" && state != "invalid" && state != "error"

		targetTemp = res.Data.ClimatisationSettings.TargetTemperatureC

		// TODO not available; use target temp to avoid wrong heating/cooling display
		outsideTemp = targetTemp

		return active, outsideTemp, targetTemp, nil
	}

	return active, outsideTemp, targetTemp, err
}

var _ api.VehicleChargeController = (*Provider)(nil)

// StartCharge implements the api.VehicleChargeController interface
func (v *Provider) StartCharge() error {
	return v.action(ActionCharge, ActionChargeStart)
}

// StopCharge implements the api.VehicleChargeController interface
func (v *Provider) StopCharge() error {
	return v.action(ActionCharge, ActionChargeStop)
}
