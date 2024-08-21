package config

import (
	"flag"
	"fmt"
)

// Config represents the configuration for the application.
type Config struct {
	HttpNetwork string
	HttpAddress string
	GrpcNetwork string
	GrpcAddress string
	LogLevel    string
}

// ParseFlags parses the command line flags into a Config and returns it.
func ParseFlags() Config {
	cfg := Config{}
	flag.StringVar(&cfg.HttpNetwork, "http-network", "tcp", "the HTTP server network (tcp, unix)")
	flag.StringVar(&cfg.GrpcNetwork, "grpc-network", "tcp", "the gRPC server network (tcp, unix)")
	flag.StringVar(&cfg.HttpAddress, "http-address", "0.0.0.0:8080", "the http network (tcp, unix)")
	flag.StringVar(&cfg.GrpcAddress, "grpc-address", "0.0.0.0:8081", "the http network (tcp, unix)")
	flag.StringVar(
		&cfg.LogLevel,
		"log-level",
		"debug",
		"the log level (trace, debug, info, warn, error, fatal, panic)",
	)
	flag.Usage = func() {
		_, _ = fmt.Fprint(flag.CommandLine.Output(), "Supported options:\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	return cfg
}
