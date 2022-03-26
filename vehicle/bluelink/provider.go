package bluelink

import (
	"time"

	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/provider"
)

const refreshTimeout = 2 * time.Minute

// Provider implements the Kia/Hyundai bluelink api.
// Based on https://github.com/Hacksore/bluelinky.
type Provider struct {
	statusG     func() (interface{}, error)
	statusLG    func() (interface{}, error)
	refreshG    func() (StatusResponse, error)
	expiry      time.Duration
	refreshTime time.Time
}

// New creates a new BlueLink API
func NewProvider(api *API, vid string, expiry, cache time.Duration) *Provider {
	v := &Provider{
		refreshG: func() (StatusResponse, error) {
			return api.StatusPartial(vid)
		},
		expiry: expiry,
	}

	v.statusG = provider.NewCached(func() (interface{}, error) {
		return v.status(
			func() (StatusLatestResponse, error) { return api.StatusLatest(vid) },
		)
	}, cache).InterfaceGetter()

	v.statusLG = provider.NewCached(func() (interface{}, error) {
		return api.StatusLatest(vid)
	}, cache).InterfaceGetter()

	return v
}

// status wraps the api status call and adds status refresh
func (v *Provider) status(statusG func() (StatusLatestResponse, error)) (VehicleStatus, error) {
	res, err := statusG()

	var ts time.Time
	if err == nil {
		ts, err = res.ResMsg.VehicleStatusInfo.VehicleStatus.Updated()
		if err != nil {
			return res.ResMsg.VehicleStatusInfo.VehicleStatus, err
		}

		// return the current value
		if time.Since(ts) <= v.expiry {
			v.refreshTime = time.Time{}
			return res.ResMsg.VehicleStatusInfo.VehicleStatus, nil
		}
	}

	// request a refresh, irrespective of a previous error
	if v.refreshTime.IsZero() {
		v.refreshTime = time.Now()

		// TODO async refresh
		res, err := v.refreshG()
		if err == nil {
			if ts, err = res.ResMsg.Updated(); err == nil && time.Since(ts) <= v.expiry {
				v.refreshTime = time.Time{}
				return res.ResMsg, nil
			}

			err = api.ErrMustRetry
		}

		return VehicleStatus{}, err
	}

	// refresh finally expired
	if time.Since(v.refreshTime) > refreshTimeout {
		v.refreshTime = time.Time{}
		if err == nil {
			err = api.ErrTimeout
		}
	} else {
		// wait for refresh, irrespective of a previous error
		err = api.ErrMustRetry
	}

	return VehicleStatus{}, err
}

var _ api.Battery = (*Provider)(nil)

// SoC implements the api.Battery interface
func (v *Provider) SoC() (float64, error) {
	res, err := v.statusG()

	if res, ok := res.(VehicleStatus); err == nil && ok {
		return res.EvStatus.BatteryStatus, nil
	}

	return 0, err
}

var _ api.ChargeState = (*Provider)(nil)

// Status implements the api.Battery interface
func (v *Provider) Status() (api.ChargeStatus, error) {
	res, err := v.statusG()

	status := api.StatusNone
	if res, ok := res.(VehicleStatus); err == nil && ok {
		status = api.StatusA
		if res.EvStatus.BatteryPlugin > 0 {
			status = api.StatusB
		}
		if res.EvStatus.BatteryCharge {
			status = api.StatusC
		}
	}

	return status, err
}

var _ api.VehicleFinishTimer = (*Provider)(nil)

// FinishTime implements the api.VehicleFinishTimer interface
func (v *Provider) FinishTime() (time.Time, error) {
	res, err := v.statusG()

	if res, ok := res.(VehicleStatus); err == nil && ok {
		remaining := res.EvStatus.RemainTime2.Atc.Value

		if remaining == 0 {
			return time.Time{}, api.ErrNotAvailable
		}

		ts, err := res.Updated()
		return ts.Add(time.Duration(remaining) * time.Minute), err
	}

	return time.Time{}, err
}

var _ api.VehicleRange = (*Provider)(nil)

// Range implements the api.VehicleRange interface
func (v *Provider) Range() (int64, error) {
	res, err := v.statusG()

	if res, ok := res.(VehicleStatus); err == nil && ok {
		if dist := res.EvStatus.DrvDistance; len(dist) == 1 {
			return int64(dist[0].RangeByFuel.EvModeRange.Value), nil
		}

		return 0, api.ErrNotAvailable
	}

	return 0, err
}

var _ api.VehicleOdometer = (*Provider)(nil)

// Range implements the api.VehicleRange interface
func (v *Provider) Odometer() (float64, error) {
	res, err := v.statusLG()

	if res, ok := res.(StatusLatestResponse); err == nil && ok {
		return res.ResMsg.VehicleStatusInfo.Odometer.Value, nil
	}

	return 0, err
}
