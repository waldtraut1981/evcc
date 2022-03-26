package charger

// LICENSE

// Copyright (c) 2019-2022 andig

// This module is NOT covered by the MIT license. All rights reserved.

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

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
	"net/http"
	"net/url"
	"strings"

	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/charger/echarge"
	"github.com/evcc-io/evcc/charger/echarge/ecb1"
	"github.com/evcc-io/evcc/meter/obis"
	"github.com/evcc-io/evcc/util"
	"github.com/evcc-io/evcc/util/request"
	"github.com/evcc-io/evcc/util/sponsor"
)

// http://apidoc.ecb1.de
// https://github.com/evcc-io/evcc/discussions/778

// HardyBarth charger implementation
type HardyBarth struct {
	*request.Helper
	uri           string
	chargecontrol int
	meter         int
	current       int64
}

func init() {
	registry.Add("hardybarth-ecb1", NewHardyBarthFromConfig)
}

// NewHardyBarthFromConfig creates a HardyBarth cPH1 charger from generic config
func NewHardyBarthFromConfig(other map[string]interface{}) (api.Charger, error) {
	cc := struct {
		URI           string
		ChargeControl int
		Meter         int
	}{
		ChargeControl: 1,
		Meter:         1,
	}

	if err := util.DecodeOther(other, &cc); err != nil {
		return nil, err
	}

	return NewHardyBarth(cc.URI, cc.ChargeControl, cc.Meter)
}

// NewHardyBarth creates HardyBarth charger
func NewHardyBarth(uri string, chargecontrol, meter int) (api.Charger, error) {
	log := util.NewLogger("ecb1")

	uri = strings.TrimSuffix(uri, "/") + "/api/v1"

	wb := &HardyBarth{
		Helper:        request.NewHelper(log),
		uri:           util.DefaultScheme(uri, "http"),
		chargecontrol: chargecontrol,
		meter:         meter,
		current:       6,
	}

	if !sponsor.IsAuthorized() {
		return nil, api.ErrSponsorRequired
	}

	uri = fmt.Sprintf("%s/chargecontrols/%d/mode", wb.uri, wb.chargecontrol)
	data := url.Values{"mode": {echarge.ModeManual}}
	err := wb.post(uri, data)

	return wb, err
}

func (wb *HardyBarth) getChargeControl() (ecb1.ChargeControl, error) {
	uri := fmt.Sprintf("%s/chargecontrols/%d", wb.uri, wb.chargecontrol)

	var res struct {
		ChargeControl struct {
			ecb1.ChargeControl
		}
	}

	err := wb.GetJSON(uri, &res)

	return res.ChargeControl.ChargeControl, err
}

// Status implements the api.Charger interface
func (wb *HardyBarth) Status() (api.ChargeStatus, error) {
	res, err := wb.getChargeControl()
	if err != nil {
		return api.StatusNone, err
	}

	if res.State == "" {
		return api.StatusNone, errors.New("invalid state- check controller type (eCB1 vs Salia)")
	}

	switch s := res.State[:1]; s {
	case "A", "B", "C":
		return api.ChargeStatus(s), nil
	default:
		return api.StatusNone, fmt.Errorf("invalid state: %s", s)
	}
}

// Enabled implements the api.Charger interface
func (wb *HardyBarth) Enabled() (bool, error) {
	res, err := wb.getChargeControl()
	if err == nil && res.Mode != echarge.ModeManual {
		err = fmt.Errorf("invalid mode: %s", res.Mode)
	}
	return res.ManualModeAmp > 0, err
}

// Enable implements the api.Charger interface
func (wb *HardyBarth) Enable(enable bool) error {
	var current int64
	if enable {
		current = wb.current
	}

	return wb.setCurrent(current)
}

func (wb *HardyBarth) post(uri string, data url.Values) error {
	resp, err := wb.PostForm(uri, data)
	if err == nil {
		defer resp.Body.Close()

		if resp.StatusCode >= http.StatusBadRequest {
			err = fmt.Errorf("invalid status: %s", resp.Status)
		}
	}

	return err
}

func (wb *HardyBarth) setCurrent(current int64) error {
	uri := fmt.Sprintf("%s/chargecontrols/%d/mode/manual/ampere", wb.uri, wb.chargecontrol)
	data := url.Values{"manualmodeamp": {fmt.Sprintf("%d", current)}}
	return wb.post(uri, data)
}

// MaxCurrent implements the api.Charger interface
func (wb *HardyBarth) MaxCurrent(current int64) error {
	err := wb.setCurrent(current)
	if err == nil {
		wb.current = current
	}
	return err
}

func (wb *HardyBarth) getMeter() (ecb1.Meter, error) {
	uri := fmt.Sprintf("%s/meters/%d", wb.uri, wb.meter)

	var res struct {
		Meter struct {
			ecb1.Meter
		}
	}

	err := wb.GetJSON(uri, &res)

	return res.Meter.Meter, err
}

var _ api.Meter = (*HardyBarth)(nil)

// CurrentPower implements the api.Meter interface
func (wb *HardyBarth) CurrentPower() (float64, error) {
	res, err := wb.getMeter()
	if err != nil {
		return 0, err
	}

	return res.Data[obis.PowerConsumption], nil
}

var _ api.MeterEnergy = (*HardyBarth)(nil)

// TotalEnergy implements the api.MeterEnergy interface
func (wb *HardyBarth) TotalEnergy() (float64, error) {
	res, err := wb.getMeter()
	if err != nil {
		return 0, err
	}

	return res.Data[obis.EnergyConsumption], nil
}

var _ api.MeterCurrent = (*HardyBarth)(nil)

// Currents implements the api.MeterCurrent interface
func (wb *HardyBarth) Currents() (float64, float64, float64, error) {
	res, err := wb.getMeter()
	if err != nil {
		return 0, 0, 0, err
	}

	return res.Data[obis.CurrentL1], res.Data[obis.CurrentL2], res.Data[obis.CurrentL3], nil
}

// var _ api.Identifier = (*HardyBarth)(nil)

// // Identify implements the api.Identifier interface
// func (wb *HardyBarth) Identify() (string, error) {
// 	return "", api.ErrNotAvailable
// }
