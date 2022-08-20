package vehicle

import (
	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/util"
	"github.com/evcc-io/evcc/util/templates"
	"gopkg.in/yaml.v3"
)

func init() {
	registry.Add("template", NewVehicleFromTemplateConfig)
}

func NewVehicleFromTemplateConfig(other map[string]interface{}) (api.Vehicle, error) {
	var cc struct {
		Template string
		Other    map[string]interface{} `mapstructure:",remain"`
	}

	if err := util.DecodeOther(other, &cc); err != nil {
		return nil, err
	}

	tmpl, err := templates.ByName(cc.Template, templates.Vehicle)
	if err != nil {
		return nil, err
	}

	b, _, err := tmpl.RenderResult(templates.TemplateRenderModeInstance, other)
	if err != nil {
		return nil, err
	}

	var instance struct {
		Type  string
		Other map[string]interface{} `yaml:",inline"`
	}

	if err := yaml.Unmarshal(b, &instance); err != nil {
		return nil, err
	}

	return NewFromConfig(instance.Type, instance.Other)
}
