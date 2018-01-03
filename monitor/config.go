package monitor

import (
	"io"
	"os"
	"time"
)

type Config struct {
	LogOutput   io.Writer
	BindAddr    string `json:"bind"`
	BindPort    int    `json:"port"`
	EthAddress  string `json:"ethaddress"`
	NodeName    string `json:"nodename"`
	RPCInterval time.Duration
}

func DefaultConfig() *Config {
	c := &Config{
		LogOutput:   os.Stderr,
		BindAddr:    "127.0.0.1",
		BindPort:    4546,
		NodeName:    "parity",
		EthAddress:  "http://127.0.0.1:8500",
		RPCInterval: time.Duration(5) * time.Second,
	}

	if hostname, err := os.Hostname(); err == nil {
		c.NodeName = hostname
	}

	return c
}
