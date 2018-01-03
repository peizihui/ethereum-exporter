package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"

	"github.com/imdario/mergo"
	"github.com/melonproject/ethereum-exporter/monitor"
)

func main() {
	if err := run(os.Args); err != nil {
		fmt.Printf("[ERR]: %v", err)
		os.Exit(1)
	}
}

func readConfigFile(path string) (*monitor.Config, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config monitor.Config
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func readConfig(args []string) (*monitor.Config, error) {

	var fileConfigPath string

	fileConfig := &monitor.Config{}
	cliConfig := &monitor.Config{}

	flag.StringVar(&fileConfigPath, "config", "", "")
	flag.StringVar(&cliConfig.EthAddress, "ethaddress", "", "")
	flag.StringVar(&cliConfig.NodeName, "nodename", "", "")
	flag.StringVar(&cliConfig.BindAddr, "bind", "", "")
	flag.IntVar(&cliConfig.BindPort, "port", 0, "")

	flag.Parse()

	if fileConfigPath != "" {
		var err error

		fileConfig, err = readConfigFile(fileConfigPath)
		if err != nil {
			return nil, err
		}
	}

	// merge everything

	config := monitor.DefaultConfig()

	err := mergo.MergeWithOverwrite(config, *fileConfig)
	if err != nil {
		return nil, err
	}

	err = mergo.MergeWithOverwrite(config, *cliConfig)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func run(args []string) error {

	config, err := readConfig(args)
	if err != nil {
		return err
	}

	// Handle interupts.
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	monitor, err := monitor.NewMonitor(config)
	if err != nil {
		return fmt.Errorf("[ERR]: Failed to create the monitor: %v", err)
	}

	if err := monitor.Start(); err != nil {
		return fmt.Errorf("[ERR]: Failed to start the monitor: %v", err)
	}

	for range c {
		monitor.Shutdown()
		break
	}

	return nil
}
