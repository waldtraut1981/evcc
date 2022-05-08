package cmd

import (
	_ "embed" // for yaml

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

//go:embed demo.yaml
var demoYaml string

func demoConfig(conf *config) {
	demo := map[string]interface{}{}

	if err := yaml.Unmarshal([]byte(demoYaml), &demo); err != nil {
		log.FATAL.Fatalf("failed decoding demo config: %+v", err)
	}

	for k, v := range demo {
		viper.Set(k, v)
	}

	if err := viper.UnmarshalExact(&conf); err != nil {
		log.FATAL.Fatalf("failed loading demo config: %v", err)
	}

	conf.Network.Port = defaultPort
}
