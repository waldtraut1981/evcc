package cmd

import (
	"fmt"

	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/server"
	"github.com/evcc-io/evcc/util"
	"github.com/evcc-io/evcc/util/request"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// meterCmd represents the meter command
var meterCmd = &cobra.Command{
	Use:   "meter [name]",
	Short: "Query configured meters",
	Run:   runMeter,
}

func init() {
	rootCmd.AddCommand(meterCmd)
	meterCmd.PersistentFlags().StringP(flagName, "n", "", fmt.Sprintf(flagNameDescription, "meter"))
	meterCmd.PersistentFlags().Bool(flagHeaders, false, flagHeadersDescription)
}

func runMeter(cmd *cobra.Command, args []string) {
	util.LogLevel(viper.GetString("log"), viper.GetStringMapString("levels"))
	log.INFO.Printf("evcc %s", server.FormattedVersion())

	// load config
	if err := loadConfigFile(cfgFile, &conf); err != nil {
		log.FATAL.Fatal(err)
	}

	// setup environment
	if err := configureEnvironment(conf); err != nil {
		log.FATAL.Fatal(err)
	}

	// full http request log
	if cmd.PersistentFlags().Lookup(flagHeaders).Changed {
		request.LogHeaders = true
	}

	// select single meter
	if err := selectByName(cmd, &conf.Meters); err != nil {
		log.FATAL.Fatal(err)
	}

	if err := cp.configureMeters(conf); err != nil {
		log.FATAL.Fatal(err)
	}

	meters := cp.meters
	if len(args) == 1 {
		arg := args[0]
		meters = map[string]api.Meter{arg: cp.Meter(arg)}
	}

	d := dumper{len: len(meters)}
	for name, v := range meters {
		d.DumpWithHeader(name, v)
	}
}
