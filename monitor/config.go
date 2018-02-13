package monitor

import (
	"io"
	"os"
	"time"
)

type ConsulConfig struct {
	Address     string   `json:"address"`
	ServiceName string   `json:"service_name"`
	Tags        []string `json:"tags"`
}

func DefaultConsulConfig() *ConsulConfig {
	return &ConsulConfig{
		Address:     "http://127.0.0.1:8500",
		ServiceName: "pool",
		Tags:        []string{"pool", "parity"},
	}
}

func (c *ConsulConfig) Merge(c1 *ConsulConfig) {
	if c1.Address != "" {
		c.Address = c1.Address
	}
	if c1.ServiceName != "" {
		c.ServiceName = c1.ServiceName
	}
	if len(c1.Tags) != 0 {
		c.Tags = c1.Tags
	}
}

type Config struct {
	LogOutput   io.Writer
	BindAddr    string `json:"bind"`
	BindPort    int    `json:"port"`
	Endpoint    string `json:"endpoint"`
	NodeName    string `json:"nodename"`
	RPCInterval time.Duration

	// Consul config
	ConsulConfig *ConsulConfig `json:"consul"`

	// Sync threashold
	SyncThreshold int
}

func DefaultConfig() *Config {
	c := &Config{
		LogOutput:     os.Stderr,
		BindAddr:      "127.0.0.1",
		BindPort:      4546,
		NodeName:      "parity",
		Endpoint:      "http://127.0.0.1:8545",
		ConsulConfig:  DefaultConsulConfig(),
		RPCInterval:   time.Duration(5) * time.Second,
		SyncThreshold: 5,
	}

	if hostname, err := os.Hostname(); err == nil {
		c.NodeName = hostname
	}

	return c
}

func (c *Config) Merge(c1 *Config) {
	if c1.BindAddr != "" {
		c.BindAddr = c1.BindAddr
	}
	if c1.BindPort != 0 {
		c.BindPort = c1.BindPort
	}
	if c1.NodeName != "" {
		c.NodeName = c1.NodeName
	}
	if c1.Endpoint != "" {
		c.Endpoint = c1.Endpoint
	}
	if c1.SyncThreshold != 0 {
		c.SyncThreshold = c1.SyncThreshold
	}

	if c1.ConsulConfig != nil {
		c.ConsulConfig.Merge(c1.ConsulConfig)
	}
}
