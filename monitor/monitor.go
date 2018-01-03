package monitor

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"
	ethclient "github.com/melonproject/ethereum-go-client/client"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	gracefulTimeout = 5 * time.Second
)

type Monitor struct {
	config    *Config
	logger    *log.Logger
	InmemSink *metrics.InmemSink
	ethClient *ethclient.Client
	HTTPAddr  net.Addr
	rcpStop   chan struct{}
	mux       *http.ServeMux
	listener  net.Listener
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

	e.HTTPAddr = &net.TCPAddr{IP: bindIP, Port: config.BindPort}

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

func (e *Monitor) Start() error {
	e.logger.Println("Staring server")

	err := e.startHttp()
	if err != nil {
		return err
	}

	go e.startRPC()

	return nil
}

func (e *Monitor) startHttp() error {

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

func (e *Monitor) wrap(handler func(resp http.ResponseWriter, req *http.Request) (interface{}, error)) http.HandlerFunc {
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

func (e *Monitor) MetricsRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
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

func (e *Monitor) Shutdown() error {
	e.logger.Println("Shutting down")

	e.listener.Close()
	e.rcpStop <- struct{}{}

	return nil
}
