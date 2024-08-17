package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/eduser25/simplefin-bridge-exporter/pkg/exporter"
	"github.com/eduser25/simplefin-bridge-exporter/pkg/logger"
	"github.com/eduser25/simplefin-bridge-exporter/pkg/simplefin"
)

const (
	defBindAddr string        = "127.0.0.1"
	defHttpPort int           = 8000
	defInterval time.Duration = time.Hour
)

var (
	setupToken     string = ""
	accessUrl      string = ""
	bindAddress    string = ""
	debug          bool   = false
	httpPort       int
	updateInterval time.Duration

	httpServ http.Server

	client simplefin.SimplefinClient
	log    = logger.NewZerologLogger()
)

func config() {
	var duration string
	var err error

	flag.StringVar(&setupToken, "setupToken", "", "SimpleFin setup Token")
	flag.StringVar(&accessUrl, "accessUrl", "", "SimpleFin access URL")
	flag.BoolVar(&debug, "debug", false, "Enable debug")
	flag.StringVar(&bindAddress, "bindAddress", defBindAddr, "Http server bind address")
	flag.IntVar(&httpPort, "port", defHttpPort, "Http server port")
	flag.StringVar(&duration, "updateInterval", defInterval.String(),
		"Update interval (golang duration string)")
	flag.Parse()

	if debug {
		logger.SetDebug()
	}

	if accessUrl == "" && setupToken == "" {
		log.Fatal().Msg("Acces URL or Setup Token required")

	}

	if accessUrl != "" && setupToken != "" {
		log.Warn().Msg("access URL and setup token provided, ignoring setup token.")
	}

	updIval, err := time.ParseDuration(duration)
	if err != nil {
		log.Fatal().Err(err).Msgf("error parsing duration")
	}
	updateInterval = updIval

	if accessUrl != "" {
		client, err = simplefin.NewSimplefinClient(accessUrl)
	} else {
		client, err = simplefin.NewSimplefinClientFromSetupToken(setupToken)
	}

	if err != nil {
		log.Fatal().Err(err).Msgf("failed to initialize simplefin client")
	}
}

func startExporterServer(e *exporter.Exporter) {
	httpMux := http.NewServeMux()
	httpMux.Handle("/metrics", promhttp.InstrumentMetricHandler(
		e.Registry,
		promhttp.HandlerFor(e.Registry, promhttp.HandlerOpts{}),
	))

	// Init & start serv
	httpServ = http.Server{
		Addr:    fmt.Sprintf(":%d", httpPort),
		Handler: httpMux,
	}

	go func() {
		err := httpServ.ListenAndServe()
		if err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("HTTP Server errored out")
		}
	}()
}

func main() {
	config()

	exporter := exporter.NewExporter()
	startExporterServer(exporter)

	log.Info().Msgf("update interval: %v", updateInterval.String())
	for {
		log.Info().Msg("polling account data")

		before := time.Now()
		accounts, err := client.GetAccounts(context.Background())
		if err != nil {
			log.Error().Err(err).Msg("failed to fetch accounts")
		} else {
			exporter.Export(accounts)
		}
		log.Info().Msgf("done, took %v", time.Since(before).String())

		time.Sleep(updateInterval)
	}

}
