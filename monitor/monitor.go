package monitor

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"
	ethclient "github.com/melonproject/ethereum-go-client/client"
)

const (
	gracefulTimeout = 5 * time.Second
)

type Monitor struct {
	config    *Config
	logger    *log.Logger
	InmemSink *metrics.InmemSink
	ethClient *ethclient.Client

	// Http server
	http *HttpServer

	rcpStop chan struct{}
}

func NewMonitor(config *Config) (*Monitor, error) {
	e := &Monitor{
		config:  config,
		rcpStop: make(chan struct{}, 1),
	}

	e.logger = log.New(config.LogOutput, "", log.LstdFlags)

	bindIP := net.ParseIP(config.BindAddr)
	if bindIP == nil {
		return nil, fmt.Errorf("Bind address '%s' is not a valid ip", bindIP)
	}

	addr := &net.TCPAddr{IP: bindIP, Port: config.BindPort}

	e.http = NewHttpServer(e.logger, e, addr)

	var err error
	e.InmemSink, err = e.setupTelemetry()
	if err != nil {
		return nil, err
	}

	return e, nil
}

func (e *Monitor) setupTelemetry() (*metrics.InmemSink, error) {
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

func (e *Monitor) Start(ctx context.Context) error {
	e.logger.Println("Staring server")

	if err := e.http.Start(ctx); err != nil {
		return err
	}

	go e.startRPC()

	return nil
}

func (e *Monitor) startRPC() {
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

func (e *Monitor) rpcCalls() error {
	var errors error

	return errors
}

func (e *Monitor) Shutdown() error {
	e.logger.Println("Shutting down")

	e.rcpStop <- struct{}{}

	return nil
}
