package wrapper

import (
	"fmt"

	"github.com/evcc-io/evcc/api"
)

// Wrapper wraps an api.Vehicle to capture initialization errors
type Wrapper struct {
	err error
}

// New creates a new Vehicle
func New(w api.Vehicle, err error) (api.Vehicle, error) {
	v := &Wrapper{
		err: fmt.Errorf("vehicle not available: %w", err),
	}

	return v, nil
}

var _ api.Vehicle = (*Wrapper)(nil)

// Title implements the api.Vehicle interface
func (v *Wrapper) Title() string {
	return "unavailable"
}

// Capacity implements the api.Vehicle interface
func (v *Wrapper) Capacity() int64 {
	return 0
}

// Phases implements the api.Vehicle interface
func (v *Wrapper) Phases() int {
	return 0
}

// Identifiers implements the api.Vehicle interface
func (v *Wrapper) Identifiers() []string {
	return nil
}

// OnIdentified implements the api.Vehicle interface
func (v *Wrapper) OnIdentified() api.ActionConfig {
	return api.ActionConfig{}
}

var _ api.Battery = (*Wrapper)(nil)

// SoC implements the api.Battery interface
func (v *Wrapper) SoC() (float64, error) {
	return 0, v.err
}
