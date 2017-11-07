package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/imdario/mergo"
	"github.com/melonproject/ethereum-go-client"

	metrics "github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	gracefulTimeout = 5 * time.Second
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

type Exporter struct {
	config    *Config
	logger    *log.Logger
	InmemSink *metrics.InmemSink
	ethClient *ethclient.Client
	HTTPAddr  net.Addr
	rcpStop   chan struct{}
	mux       *http.ServeMux
	listener  net.Listener
}

func NewExporter(config *Config) (*Exporter, error) {
	e := &Exporter{
		config:  config,
		rcpStop: make(chan struct{}, 1),
	}

	e.logger = log.New(config.LogOutput, "", log.LstdFlags)

	bindIP := net.ParseIP(config.BindAddr)
	if bindIP == nil {
		return nil, fmt.Errorf("Bind address '%s' is not a valid ip", bindIP)
	}

	e.HTTPAddr = &net.TCPAddr{IP: bindIP, Port: config.BindPort}

	var err error
	e.InmemSink, err = e.setupTelemetry()
	if err != nil {
		return nil, err
	}

	return e, nil
}

func (e *Exporter) setupTelemetry() (*metrics.InmemSink, error) {
	// Prepare metrics

	memSink := metrics.NewInmemSink(10*time.Second, time.Minute)
	metrics.DefaultInmemSignal(memSink)

	metricsConf := metrics.DefaultConfig("parity")

	var sinks metrics.FanoutSink

	prom, err := prometheus.NewPrometheusSink()
	if err != nil {
		panic(err)
	}

	sinks = append(sinks, prom)

	if len(sinks) > 0 {
		sinks = append(sinks, memSink)
		metrics.NewGlobal(metricsConf, sinks)
	} else {
		metricsConf.EnableHostname = false
		metrics.NewGlobal(metricsConf, memSink)
	}

	return memSink, nil
}

func (e *Exporter) Start() error {
	e.logger.Println("Staring server")

	err := e.startHttp()
	if err != nil {
		return err
	}

	go e.startRPC()

	return nil
}

func (e *Exporter) startHttp() error {

	l, err := net.Listen("tcp", e.HTTPAddr.String())
	if err != nil {
		return fmt.Errorf("failed to start listner on %s: %v", e.HTTPAddr.String(), err)
	}

	e.listener = l

	e.mux = http.NewServeMux()
	e.mux.Handle("/metrics", e.wrap(e.MetricsRequest))

	go http.Serve(l, e.mux)

	e.logger.Printf("Http api running on %s", e.HTTPAddr.String())

	return nil
}

func (e *Exporter) wrap(handler func(resp http.ResponseWriter, req *http.Request) (interface{}, error)) http.HandlerFunc {
	return func(resp http.ResponseWriter, req *http.Request) {
		handleErr := func(err error) {
			resp.WriteHeader(http.StatusInternalServerError)
			resp.Write([]byte(err.Error()))
		}

		obj, err := handler(resp, req)
		if err != nil {
			handleErr(err)
			return
		}

		if obj == nil {
			return
		}

		buf, err := json.Marshal(obj)
		if err != nil {
			handleErr(err)
			return
		}

		resp.Header().Set("Content-Type", "application/json")
		resp.Write(buf)
	}
}

func (e *Exporter) startRPC() {
	e.ethClient = ethclient.NewClient(e.config.EthAddress)
	e.logger.Printf("Ethereum client address: %s", e.config.EthAddress)

	for {
		select {
		case <-time.After(5 * time.Second):
			err := e.rpcCalls()
			if err != nil {
				e.logger.Printf("[ERR]: Failed to make rpc calls to parity: %v", err)
			}

		case <-e.rcpStop:
			return
		}
	}
}

func (e *Exporter) rpcCalls() error {
	var errors error

	// Peers

	peers, err := e.ethClient.NetPeerCount()
	if err != nil {
		errors = multierror.Append(errors, err)
	} else {
		metrics.SetGauge([]string{"peers"}, float32(peers))
	}

	// BlockNumber

	blockNumber, err := e.ethClient.EthBlockNumber()
	if err != nil {
		errors = multierror.Append(errors, err)
	} else {
		metrics.SetGauge([]string{"blockNumber"}, float32(blockNumber))
	}

	// GassPrice

	gassPrice, err := e.ethClient.EthGassPrice()
	if err != nil {
		errors = multierror.Append(errors, err)
	} else {
		metrics.SetGauge([]string{"gassPrice"}, float32(gassPrice))
	}

	// HashRate

	hashRate, err := e.ethClient.EthHashRate()
	if err != nil {
		errors = multierror.Append(errors, err)
	} else {
		metrics.SetGauge([]string{"hashRate"}, float32(hashRate))
	}

	return errors
}

func (e *Exporter) MetricsRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "GET" {
		return nil, fmt.Errorf("Incorrect method. Found %s, only GET available", req.Method)
	}

	if format := req.URL.Query().Get("format"); format == "prometheus" {
		handler := promhttp.Handler()
		handler.ServeHTTP(resp, req)
		return nil, nil
	}

	return e.InmemSink.DisplayMetrics(resp, req)
}

func (e *Exporter) Shutdown() error {
	e.logger.Println("Shutting down")

	e.listener.Close()
	e.rcpStop <- struct{}{}

	return nil
}

func main() {
	if err := run(os.Args); err != nil {
		fmt.Printf("[ERR]: %v", err)
		os.Exit(1)
	}
}

func readConfigFile(path string) (*Config, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func readConfig(args []string) (*Config, error) {

	var fileConfigPath string

	fileConfig := &Config{}
	cliConfig := &Config{}

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

	config := DefaultConfig()

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

	exporter, err := NewExporter(config)
	if err != nil {
		return fmt.Errorf("[ERR]: Failed to create the exporter: %v", err)
	}

	if err := exporter.Start(); err != nil {
		return fmt.Errorf("[ERR]: Failed to start the exporter: %v", err)
	}

	for range c {
		exporter.Shutdown()
		break
	}

	return nil
}
