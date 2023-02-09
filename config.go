package main

import (
	"flag"
	"os"

	logrus "github.com/sirupsen/logrus"
)

type Config struct {
	PostgresEndpoint string
}

// By default the release is a custom build. CI takes care of upgrading it with
// go build -v -ldflags="-X 'github.com/xxx/yyy/config.ReleaseVersion=x.y.z'"
var ReleaseVersion = "custom-build"

func NewCliConfig() (*Config, error) {
	var version = flag.Bool("version", false, "Prints the release version and exits")
	var postgresEndpoint = flag.String("postgres-endpoint", "", "Postgres endpoint")

	flag.Parse()

	if *version {
		logrus.Info("Version: ", ReleaseVersion)
		os.Exit(0)
	}

	conf := &Config{
		PostgresEndpoint: *postgresEndpoint,
	}
	logConfig(conf)
	return conf, nil
}

func logConfig(cfg *Config) {
	logrus.WithFields(logrus.Fields{
		"PostgresEndpoint": cfg.PostgresEndpoint,
	}).Info("Cli Config:")
}
