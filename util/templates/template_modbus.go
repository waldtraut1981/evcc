package templates

import (
	_ "embed"
	"fmt"
	"strings"
)

//go:embed modbus.tpl
var modbusTmpl string

// add the modbus params to the template
func (t *Template) ModbusParams(modbusType string, values map[string]interface{}) {
	if len(t.ModbusChoices()) == 0 {
		return
	}

	if modbusType != "" {
		values[ParamModbus] = modbusType
	}

	if values[ParamModbus] == nil || values[ParamModbus] == "" {
		return
	}

	modbusParams := t.ConfigDefaults.Config.Modbus.Types[values[ParamModbus].(string)].Params
	// add the modbus params at the beginning
	t.Params = append(modbusParams, t.Params...)
}

// set the modbus values required from modbus.tpl and and the template to the render
func (t *Template) ModbusValues(renderMode string, setDefaults bool, values map[string]interface{}) map[string]interface{} {
	choices := t.ModbusChoices()
	if len(choices) == 0 {
		return values
	}

	// only add the template once, when testing multiple usages, it might already be present
	if !strings.Contains(t.Render, modbusTmpl) {
		t.Render = fmt.Sprintf("%s\n%s", t.Render, modbusTmpl)
	}

	if !setDefaults {
		return values
	}

	modbusConfig := t.ConfigDefaults.Config.Modbus
	_, modbusParam := t.ParamByName(ParamModbus)

	modbusInterfaces := []string{}
	for _, choice := range choices {
		modbusInterfaces = append(modbusInterfaces, modbusConfig.Interfaces[choice]...)
	}

	for _, iface := range modbusInterfaces {
		typeParams := modbusConfig.Types[iface].Params
		for _, p := range typeParams {
			values[p.Name] = p.DefaultValue(renderMode)

			var defaultValue string

			switch p.Name {
			case ModbusParamNameId:
				if modbusParam.ID != 0 {
					defaultValue = fmt.Sprintf("%d", modbusParam.ID)
				}
			case ModbusParamNamePort:
				if modbusParam.Port != 0 {
					defaultValue = fmt.Sprintf("%d", modbusParam.Port)
				}
			case ModbusParamNameBaudrate:
				if modbusParam.Baudrate != 0 {
					defaultValue = fmt.Sprintf("%d", modbusParam.Baudrate)
				}
			case ModbusParamNameComset:
				if modbusParam.Comset != "" {
					defaultValue = modbusParam.Comset
				}
			}

			if defaultValue == "" {
				continue
			}

			if renderMode == TemplateRenderModeInstance {
				t.SetParamDefault(p.Name, defaultValue)
			} else {
				values[p.Name] = defaultValue
			}

		}
		if renderMode == TemplateRenderModeDocs {
			values[iface] = true
		}
	}

	return values
}
