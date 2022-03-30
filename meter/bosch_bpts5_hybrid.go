package meter

// LICENSE

// Bosch is the Bosch BPT-S 5 Hybrid meter

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/meter/bosch"
	"github.com/evcc-io/evcc/util"
)

// Example config:
// meters:
// - name: bosch_grid
//   type: bosch-bpts5-hybrid
//   uri: http://192.168.178.22
//   usage: grid
// - name: bosch_pv
//   type: bosch-bpts5-hybrid
//   uri: http://192.168.178.22
//   usage: pv
// - name: bosch_battery
//   type: bosch-bpts5-hybrid
//   uri: http://192.168.178.22
//   usage: battery

type BoschBpts5Hybrid struct {
	api                bosch.API
	usage              string
	currentTotalEnergy float64
	logger             *util.Logger
}

func init() {
	registry.Add("bosch-bpts5-hybrid", NewBoschBpts5HybridFromConfig)
}

var boschApiInstances map[string]bosch.API = make(map[string]bosch.API)

//go:generate go run ../cmd/tools/decorate.go -f decorateBoschBpts5Hybrid -b api.Meter -t "api.MeterEnergy,TotalEnergy,func() (float64, error)" -t "api.Battery,SoC,func() (float64, error)"

// NewBoschBpts5HybridFromConfig creates a Bosch BPT-S 5 Hybrid Meter from generic config
func NewBoschBpts5HybridFromConfig(other map[string]interface{}) (api.Meter, error) {
	cc := struct {
		URI   string
		Usage string
		Cache time.Duration
	}{}

	if err := util.DecodeOther(other, &cc); err != nil {
		return nil, err
	}

	if cc.Usage == "" {
		return nil, errors.New("missing usage")
	}

	_, err := url.Parse(cc.URI)
	if err != nil {
		return nil, fmt.Errorf("%s is invalid: %s", cc.URI, err)
	}

	return NewBoschBpts5Hybrid(cc.URI, cc.Usage, cc.Cache)
}

// NewBoschBpts5Hybrid creates a Bosch BPT-S 5 Hybrid Meter
func NewBoschBpts5Hybrid(uri, usage string, cache time.Duration) (api.Meter, error) {
	log := util.NewLogger("bosch-bpts5-hybrid")

	var apiInstance bosch.API
	apiInstance = boschApiInstances[uri]

	if apiInstance == nil {
		apiInstance = bosch.NewLocal(log, uri, cache)
		err := apiInstance.Login()

		if err != nil {
			return nil, err
		}

		boschApiInstances[uri] = apiInstance
	}

	m := &BoschBpts5Hybrid{
		api:                apiInstance,
		usage:              strings.ToLower(usage),
		currentTotalEnergy: 0.0,
		logger:             log,
	}

	// decorate api.MeterEnergy
	var totalEnergy func() (float64, error)
	if m.usage == "grid" || m.usage == "pv" {
		totalEnergy = m.totalEnergy
	}

	// decorate api.BatterySoC
	var batterySoC func() (float64, error)
	if usage == "battery" {
		batterySoC = m.batterySoC
	}

	return decorateBoschBpts5Hybrid(m, totalEnergy, batterySoC), nil
}

// CurrentPower implements the api.Meter interface
func (m *BoschBpts5Hybrid) CurrentPower() (float64, error) {
	status, err := m.api.Status()

	if err != nil {
		return 0.0, err
	}

	if m.usage == "grid" {
		if status.SellToGrid > 0.0 {
			return -1.0 * status.SellToGrid, nil
		} else {
			return status.BuyFromGrid, nil
		}
	}
	if m.usage == "pv" {
		return status.PvPower, nil
	}
	if m.usage == "battery" {
		if status.BatteryChargePower > 0.0 {
			return -1.0 * status.BatteryChargePower, nil
		} else {
			return status.BatteryDischargePower, nil
		}
	}
	return 0.0, nil
}

// totalEnergy implements the api.MeterEnergy interface
func (m *BoschBpts5Hybrid) totalEnergy() (float64, error) {
	return m.currentTotalEnergy, nil
}

// batterySoC implements the api.Battery interface
func (m *BoschBpts5Hybrid) batterySoC() (float64, error) {
	status, err := m.api.Status()

	if err != nil {
		return 0.0, err
	}

	return status.CurrentBatterySoc, nil
}
