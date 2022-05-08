package cmd

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/cmd/shutdown"
	"github.com/evcc-io/evcc/server"
	"github.com/evcc-io/evcc/util"
	"github.com/evcc-io/evcc/util/request"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// cmdEcho.AddCommand

// chargerRampCmd represents the charger command
var chargerRampCmd = &cobra.Command{
	Use:   "ramp [name]",
	Short: "Ramp current from 6..16A in configurable steps",
	Run:   runChargerRamp,
}

func init() {
	chargerCmd.AddCommand(chargerRampCmd)

	chargerRampCmd.PersistentFlags().StringP(flagName, "n", "", fmt.Sprintf(flagNameDescription, "charger"))
	chargerRampCmd.PersistentFlags().Bool(flagHeaders, false, flagHeadersDescription)
	chargerRampCmd.PersistentFlags().StringP(flagDigits, "", "0", "fractional digits (0..2)")
	chargerRampCmd.PersistentFlags().StringP(flagDelay, "", "1s", "ramp delay")
}

func ramp(c api.Charger, digits int, delay time.Duration) {
	steps := math.Pow10(digits)
	delta := 1 / steps

	fmt.Printf("delay:\t%s\n", delay)
	fmt.Printf("\n%6s\t%6s\n", "I (A)", "P (W)")

	if err := c.Enable(true); err != nil {
		log.ERROR.Fatalln(err)
	}
	defer func() { _ = c.Enable(false) }()

	for i := 6.0; i <= 16; {
		var err error

		if cc, ok := c.(api.ChargerEx); ok {
			err = cc.MaxCurrentMillis(i)
		} else {
			err = c.MaxCurrent(int64(i))
		}

		time.Sleep(delay)

		var p float64
		if cc, ok := c.(api.Meter); err == nil && ok {
			p, err = cc.CurrentPower()
		}

		if err != nil {
			log.ERROR.Fatalln(err)
		}

		fmt.Printf("%6.3f\t%6.0f\n", i, p)

		i += delta
	}
}

func runChargerRamp(cmd *cobra.Command, args []string) {
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

	// select single charger
	if err := selectByName(cmd, &conf.Chargers); err != nil {
		log.FATAL.Fatal(err)
	}

	if err := cp.configureChargers(conf); err != nil {
		log.FATAL.Fatal(err)
	}

	stopC := make(chan struct{})
	go shutdown.Run(stopC)

	chargers := cp.chargers
	if len(args) == 1 {
		arg := args[0]
		chargers = map[string]api.Charger{arg: cp.Charger(arg)}
	}

	digits, err := strconv.Atoi(cmd.PersistentFlags().Lookup(flagDigits).Value.String())
	if err != nil {
		log.ERROR.Fatalln(err)
	}

	delay, err := time.ParseDuration(cmd.PersistentFlags().Lookup(flagDelay).Value.String())
	if err != nil {
		log.ERROR.Fatalln(err)
	}

	for _, c := range chargers {
		if _, ok := c.(api.ChargerEx); digits > 0 && !ok {
			log.ERROR.Fatalln("charger does not support mA control")
		}
		ramp(c, digits, delay)
	}

	close(stopC)
	<-shutdown.Done()
}
