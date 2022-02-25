package cmd

import (
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof" // pprof handler
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/evcc-io/evcc/cmd/shutdown"
	"github.com/evcc-io/evcc/server"
	"github.com/evcc-io/evcc/server/updater"
	"github.com/evcc-io/evcc/util"
	"github.com/evcc-io/evcc/util/pipe"
	"github.com/evcc-io/evcc/util/sponsor"
	"github.com/grandcat/zeroconf"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	log     = util.NewLogger("main")
	cfgFile string

	ignoreErrors = []string{"warn", "error", "fatal"} // don't add to cache
	ignoreMqtt   = []string{"auth", "releaseNotes"}   // excessive size may crash certain brokers
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:     "evcc",
	Short:   "EV Charge Controller",
	Version: server.FormattedVersion(),
	Run:     run,
}

func bind(cmd *cobra.Command, flag string) {
	if err := viper.BindPFlag(flag, cmd.PersistentFlags().Lookup(flag)); err != nil {
		panic(err)
	}
}

func configureCommand(cmd *cobra.Command) {
	cmd.PersistentFlags().StringP(
		"log", "l",
		"error",
		"Log level (fatal, error, warn, info, debug, trace)",
	)
	bind(cmd, "log")

	cmd.PersistentFlags().StringVarP(&cfgFile,
		"config", "c",
		"",
		"Config file (default \"~/evcc.yaml\" or \"/etc/evcc.yaml\")",
	)
	cmd.PersistentFlags().BoolP(
		"help", "h",
		false,
		"Help for "+cmd.Name(),
	)
}

func init() {
	cobra.OnInitialize(initConfig)
	configureCommand(rootCmd)

	rootCmd.PersistentFlags().StringP(
		"uri", "u",
		"0.0.0.0:7070",
		"Listen address",
	)
	bind(rootCmd, "uri")

	rootCmd.PersistentFlags().DurationP(
		"interval", "i",
		10*time.Second,
		"Update interval",
	)
	bind(rootCmd, "interval")

	rootCmd.PersistentFlags().Bool(
		"metrics",
		false,
		"Expose metrics",
	)
	bind(rootCmd, "metrics")

	rootCmd.PersistentFlags().Bool(
		"profile",
		false,
		"Expose pprof profiles",
	)
	bind(rootCmd, "profile")
}

// initConfig reads in config file and ENV variables if set
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag
		viper.SetConfigFile(cfgFile)
	} else {
		// Search for config in home directory if available
		if home, err := os.UserHomeDir(); err == nil {
			viper.AddConfigPath(home)
		}

		// Search config in home directory with name "mbmd" (without extension).
		viper.AddConfigPath(".")    // optionally look for config in the working directory
		viper.AddConfigPath("/etc") // path to look for the config file in

		viper.SetConfigName("evcc")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in
	if err := viper.ReadInConfig(); err == nil {
		// using config file
		cfgFile = viper.ConfigFileUsed()
	} else if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
		// parsing failed - exit
		fmt.Println(err)
		os.Exit(1)
	} else {
		// not using config file
		cfgFile = ""
	}
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) {
	util.LogLevel(viper.GetString("log"), viper.GetStringMapString("levels"))
	log.INFO.Printf("evcc %s", server.FormattedVersion())

	// load config and re-configure logging after reading config file
	conf, err := loadConfigFile(cfgFile)
	if err != nil {
		log.ERROR.Println("missing evcc config - switching into demo mode")
		conf = demoConfig()
	}

	util.LogLevel(viper.GetString("log"), viper.GetStringMapString("levels"))

	uri := viper.GetString("uri")
	log.INFO.Println("listening at", uri)

	// setup environment
	if err := configureEnvironment(conf); err != nil {
		log.FATAL.Fatal(err)
	}

	// setup loadpoints
	cp.TrackVisitors() // track duplicate usage

	site, err := configureSiteAndLoadpoints(conf)
	if err != nil {
		log.FATAL.Fatal(err)
	}

	// start broadcasting values
	tee := &util.Tee{}

	// value cache
	cache := util.NewCache()
	go cache.Run(pipe.NewDropper(ignoreErrors...).Pipe(tee.Attach()))

	// setup database
	if conf.Influx.URL != "" {
		configureDatabase(conf.Influx, site.LoadPoints(), tee.Attach())
	}

	// setup mqtt publisher
	if conf.Mqtt.Broker != "" {
		publisher := server.NewMQTT(conf.Mqtt.RootTopic())
		go publisher.Run(site, pipe.NewDropper(ignoreMqtt...).Pipe(tee.Attach()))
	}

	// create webserver
	socketHub := server.NewSocketHub()
	httpd := server.NewHTTPd(uri, site, socketHub, cache)

	// announce webserver on mDNS
	if _, port, err := net.SplitHostPort(uri); err == nil {
		if portInt, err := strconv.Atoi(port); err == nil {
			if zc, err := zeroconf.RegisterProxy("evcc Website", "_http._tcp", "local.", portInt, "evcc", nil, []string{}, nil); err == nil {
				shutdown.Register(zc.Shutdown)
			} else {
				log.ERROR.Printf("mDNS announcement: %s", err)
			}
		}
	}

	// metrics
	if viper.GetBool("metrics") {
		httpd.Router().Handle("/metrics", promhttp.Handler())
	}

	// pprof
	if viper.GetBool("profile") {
		httpd.Router().PathPrefix("/debug/").Handler(http.DefaultServeMux)
	}

	// start HEMS server
	if conf.HEMS.Type != "" {
		hems := configureHEMS(conf.HEMS, site, httpd)
		go hems.Run()
	}

	// publish to UI
	go socketHub.Run(tee.Attach(), cache)

	// setup values channel
	valueChan := make(chan util.Param)
	go tee.Run(valueChan)

	// expose sponsor to UI
	if sponsor.Subject != "" {
		valueChan <- util.Param{Key: "sponsor", Val: sponsor.Subject}
	}

	// allow web access for vehicles
	cp.webControl(httpd, valueChan)

	// version check
	go updater.Run(log, httpd, tee, valueChan)

	// capture log messages for UI
	util.CaptureLogs(valueChan)

	// setup messaging
	pushChan := configureMessengers(conf.Messaging, cache)

	// set channels
	site.DumpConfig()
	site.Prepare(valueChan, pushChan)

	stopC := make(chan struct{})
	go shutdown.Run(stopC)

	siteC := make(chan struct{})
	go func() {
		site.Run(stopC, conf.Interval)
		close(siteC)
	}()

	// uds health check listener
	go server.HealthListener(site, siteC)

	// catch signals
	go func() {
		signalC := make(chan os.Signal, 1)
		signal.Notify(signalC, os.Interrupt, syscall.SIGTERM)

		<-signalC    // wait for signal
		close(stopC) // signal loop to end

		exitC := make(chan struct{})
		wg := new(sync.WaitGroup)
		wg.Add(2)

		// wait for main loop and shutdown functions to finish
		go func() { <-shutdown.Done(conf.Interval); wg.Done() }()
		go func() { <-siteC; wg.Done() }()
		go func() { wg.Wait(); close(exitC) }()

		select {
		case <-exitC: // wait for loop to end
		case <-time.NewTimer(conf.Interval).C: // wait max 1 period
		}

		os.Exit(1)
	}()

	log.FATAL.Println(httpd.ListenAndServe())
}
