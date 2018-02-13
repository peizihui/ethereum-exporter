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
	Endpoint    string `json:"endpoint"`
	NodeName    string `json:"nodename"`
	RPCInterval time.Duration

	// Consul config
	ConsulAddress     string
	ConsulServiceName string

	// Sync threashold
	SyncThreshold int
}

func DefaultConfig() *Config {
	c := &Config{
		LogOutput:         os.Stderr,
		BindAddr:          "127.0.0.1",
		BindPort:          4546,
		NodeName:          "parity",
		Endpoint:          "http://127.0.0.1:8545",
		RPCInterval:       time.Duration(5) * time.Second,
		ConsulAddress:     "http://127.0.0.1:8500",
		ConsulServiceName: "pool",
		SyncThreshold:     5,
	}

	if hostname, err := os.Hostname(); err == nil {
		c.NodeName = hostname
	}

	return c
}
