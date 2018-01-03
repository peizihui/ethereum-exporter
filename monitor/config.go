package monitor

import (
	"io"
	"os"
)

type Config struct {
	LogOutput  io.Writer
	BindAddr   string `json:"bind"`
	BindPort   int    `json:"port"`
	EthAddress string `json:"ethaddress"`
	NodeName   string `json:"nodename"`
}

func DefaultConfig() *Config {
	c := &Config{
		LogOutput:  os.Stderr,
		BindAddr:   "127.0.0.1",
		BindPort:   4546,
		NodeName:   "parity",
		EthAddress: "localhost:8545",
	}

	return c
}
