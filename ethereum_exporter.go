package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"

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

	config := monitor.DefaultConfig()

	fileConfig := &monitor.Config{
		ConsulConfig: &monitor.ConsulConfig{},
	}

	cliConfig := &monitor.Config{
		ConsulConfig: &monitor.ConsulConfig{},
	}

	flag.StringVar(&fileConfigPath, "config", "", "")
	flag.StringVar(&cliConfig.Endpoint, "endpoint", "", "")
	flag.StringVar(&cliConfig.NodeName, "nodename", "", "")
	flag.StringVar(&cliConfig.BindAddr, "bind", "", "")
	flag.IntVar(&cliConfig.BindPort, "port", 0, "")
	flag.IntVar(&cliConfig.SyncThreshold, "threshold", 5, "")

	flag.Parse()

	if fileConfigPath != "" {
		var err error

		fileConfig, err = readConfigFile(fileConfigPath)
		if err != nil {
			return nil, err
		}

		config.Merge(fileConfig)
	}

	config.Merge(cliConfig)
	return config, nil
}

func run(args []string) error {

	ctx := context.Background()

	config, err := readConfig(args)
	if err != nil {
		return fmt.Errorf("Failed to read config: %v", err)
	}

	prettyConfig, err := json.MarshalIndent(config, "", "\t")
	if err != nil {
		return fmt.Errorf("Failed to prettify config: %v", err)
	}

	fmt.Println(string(prettyConfig))

	// Handle interupts.
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	monitor, err := monitor.NewMonitor(config)
	if err != nil {
		return fmt.Errorf("Failed to create the monitor: %v", err)
	}

	if err := monitor.Start(ctx); err != nil {
		return fmt.Errorf("Failed to start the monitor: %v", err)
	}

	for range c {
		ctx.Done()
		break
	}

	return nil
}
