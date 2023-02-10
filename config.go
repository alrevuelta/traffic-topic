package main

import (
	"flag"
	"os"

	logrus "github.com/sirupsen/logrus"
)

type Config struct {
	PostgresEndpoint string
	ResetTable       bool
	TickSeconds      int
	DryRun           bool
}

// By default the release is a custom build. CI takes care of upgrading it with
// go build -v -ldflags="-X 'github.com/xxx/yyy/config.ReleaseVersion=x.y.z'"
var ReleaseVersion = "custom-build"

func NewCliConfig() (*Config, error) {
	var version = flag.Bool("version", false, "Prints the release version and exits")
	var postgresEndpoint = flag.String("postgres-endpoint", "", "Postgres endpoint")
	var resetTable = flag.Bool("reset-table", false, "If true resets the table (default: false)")
	var tickSeconds = flag.Int("tick-seconds", 1800, "Tick seconds (default: 60s)")
	var dryRun = flag.Bool("dry-run", false, "If true does not write to the database (default: false)")

	flag.Parse()

	if *version {
		logrus.Info("Version: ", ReleaseVersion)
		os.Exit(0)
	}

	conf := &Config{
		PostgresEndpoint: *postgresEndpoint,
		ResetTable:       *resetTable,
		TickSeconds:      *tickSeconds,
		DryRun:           *dryRun,
	}
	logConfig(conf)
	return conf, nil
}

func logConfig(cfg *Config) {
	logrus.WithFields(logrus.Fields{
		"PostgresEndpoint": cfg.PostgresEndpoint,
		"ResetTable":       cfg.ResetTable,
		"TickSeconds":      cfg.TickSeconds,
		"DryRun":           cfg.DryRun,
	}).Info("Cli Config:")
}
