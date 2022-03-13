package charger

import (
	"testing"

	"github.com/evcc-io/evcc/util/templates"
	"github.com/evcc-io/evcc/util/test"
	"github.com/thoas/go-funk"
)

var acceptable = []string{
	"invalid plugin type: ...",
	"missing mqtt broker configuration",
	"mqtt not configured",
	"invalid charger type: nrgkick-bluetooth",
	"NRGKick bluetooth is only supported on linux",
	"invalid pin:",
	"hciconfig provided no response",
	"connect: no route to host",
	"connect: connection refused",
	"error connecting: Network Error",
	"i/o timeout",
	"recv timeout",
	"(Client.Timeout exceeded while awaiting headers)",
	"can only have either uri or device", // modbus
	"sponsorship required, see https://github.com/evcc-io/evcc#sponsorship",
	"eebus not configured",
	"Get \"http://192.0.2.2/shelly\": context deadline exceeded",        // shelly
	"unexpected status: 400",                                            // easee
	"Get \"http://192.0.2.2/getParameters\": context deadline exceeded", // evsewifi
}

func TestTemplates(t *testing.T) {
	for _, tmpl := range templates.ByClass(templates.Charger) {
		tmpl := tmpl

		// set default values for all params
		values := tmpl.Defaults(templates.TemplateRenderModeUnitTest)

		// set the template value which is needed for rendering
		values["template"] = tmpl.Template

		// set modbus default test values
		if values[templates.ParamModbus] != nil {
			modbusChoices := tmpl.ModbusChoices()
			// we only test one modbus setup
			if funk.ContainsString(modbusChoices, templates.ModbusChoiceTCPIP) {
				values[templates.ModbusKeyTCPIP] = true
			} else {
				values[templates.ModbusKeyRS485TCPIP] = true
			}
			values = tmpl.ModbusValues(templates.TemplateRenderModeInstance, true, values)
		}

		templates.RenderTest(t, tmpl, values, func(values map[string]interface{}) {
			if _, err := NewFromConfig("template", values); err != nil && !test.Acceptable(err, acceptable) {
				t.Error(err)
			}
		})
	}
}
