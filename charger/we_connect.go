package charger

import (
	"fmt"

	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/util"
)

// WeConnect charger implementation
type WeConnect struct {
	log     *util.Logger
	current uint16
}

func init() {
	registry.Add("weconnect", NewWeConnectFromConfig)
}

// NewWeConnectFromConfig creates a WeConnect (virtual) charger from generic config
func NewWeConnectFromConfig(other map[string]interface{}) (api.Charger, error) {
	return NewWeConnect()
}

// NewWeConnect creates WeConnect (virtual) charger
func NewWeConnect() (api.Charger, error) {
	log := util.NewLogger("weconnect")

	wb := &WeConnect{
		log:     log,
		current: 60, // assume min current
	}

	return wb, nil
}

// Status implements the api.Charger interface
func (wb *WeConnect) Status() (api.ChargeStatus, error) {
	return api.StatusB, nil
}

// Enabled implements the api.Charger interface
func (wb *WeConnect) Enabled() (bool, error) {
	return true, nil
}

// Enable implements the api.Charger interface
func (wb *WeConnect) Enable(enable bool) error {
	return nil
}

// MaxCurrent implements the api.Charger interface
func (wb *WeConnect) MaxCurrent(current int64) error {
	return nil
}

var _ api.ChargerEx = (*WeConnect)(nil)

// MaxCurrentMillis implements the api.ChargerEx interface
func (wb *WeConnect) MaxCurrentMillis(current float64) error {
	return nil
}

var _ api.Meter = (*WeConnect)(nil)

// CurrentPower implements the api.Meter interface
func (wb *WeConnect) CurrentPower() (float64, error) {
	return float64(0), nil
}

var _ api.MeterEnergy = (*WeConnect)(nil)

// TotalEnergy implements the api.MeterEnergy interface
func (wb *WeConnect) TotalEnergy() (float64, error) {
	return float64(0) / 1e3, nil
}

var _ api.MeterCurrent = (*WeConnect)(nil)

// Currents implements the api.MeterCurrent interface
func (wb *WeConnect) Currents() (float64, float64, float64, error) {
	var currents []float64
	for range hecRegCurrents {
		currents = append(currents, float64(0)/10)
	}

	return currents[0], currents[1], currents[2], nil
}

var _ api.Diagnosis = (*WeConnect)(nil)

// Diagnose implements the api.Diagnosis interface
func (wb *WeConnect) Diagnose() {
	fmt.Printf("Test")
}
