package monitor

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"net"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"
	"github.com/hashicorp/go-multierror"
)

type Monitor struct {
	config    *Config
	logger    *log.Logger
	InmemSink *metrics.InmemSink

	// Etherscan
	etherscan *Etherscan

	// Ethereum client
	ethClient *EthClient

	// Http server
	http *HttpServer

	// Last block number
	lastBlock *Block
	synced    bool
}

func NewMonitor(config *Config) (*Monitor, error) {
	m := &Monitor{
		config: config,
		synced: false,
	}

	m.logger = log.New(config.LogOutput, "", log.LstdFlags)

	bindIP := net.ParseIP(config.BindAddr)
	if bindIP == nil {
		return nil, fmt.Errorf("Bind address '%s' is not a valid ip", bindIP)
	}

	if err := m.setupApis(); err != nil {
		panic(err)
	}

	addr := &net.TCPAddr{IP: bindIP, Port: config.BindPort}

	m.http = NewHttpServer(m.logger, m, addr)

	var err error

	m.InmemSink, err = m.setupTelemetry()
	if err != nil {
		return nil, err
	}

	return m, nil
}

func (m *Monitor) setupApis() error {

	m.ethClient = NewEthClient(m.config.EthAddress)

	chain, err := m.ethClient.Chain()
	if err != nil {
		return err
	}

	var url string
	switch chain {
	case "kovan":
		url = "https://kovan.etherscan.io/api?module=proxy&action=eth_blockNumber"
	case "foundation":
		url = "https://api.etherscan.io/api?module=proxy&action=eth_blockNumber"
	default:
		return fmt.Errorf("Chain %s not found. 'kovan' and 'foundation' are the only valid options", chain)
	}

	m.logger.Printf("Using chain %s", chain)
	m.etherscan = NewEtherscan(url)

	return nil
}

func (e *Monitor) setupTelemetry() (*metrics.InmemSink, error) {
	// Prepare metrics

	memSink := metrics.NewInmemSink(10*time.Second, time.Minute)
	metrics.DefaultInmemSignal(memSink)

	metricsConf := metrics.DefaultConfig(e.config.NodeName)

	var sinks metrics.FanoutSink

	prom, err := prometheus.NewPrometheusSink()
	if err != nil {
		return nil, err
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

func Sub(x, y *big.Int) *big.Int {
	return big.NewInt(0).Sub(x, y)
}

func (m *Monitor) Start(ctx context.Context) error {
	m.logger.Println("Staring monitor")

	if err := m.http.Start(ctx); err != nil {
		return err
	}

	go m.start(ctx)
	return nil
}

func (m *Monitor) start(ctx context.Context) {
	for {
		select {
		case <-time.After(m.config.RPCInterval):
			// RPC calls
			if err := m.gatherMetrics(); err != nil {
				panic(err)
			}
		case <-ctx.Done():
			m.logger.Println("Monitor shutting down")
		}
	}
}

func (m *Monitor) gatherMetrics() error {
	var errors error

	// Peers

	peers, err := m.ethClient.PeerCount()
	if err != nil {
		errors = multierror.Append(errors, err)
	} else {
		metrics.SetGauge([]string{"peers"}, float32(peers))
	}

	// BlockNumber

	blockNumber, err := m.ethClient.BlockNumber()
	if err != nil {
		errors = multierror.Append(errors, err)
	} else {
		metrics.SetGauge([]string{"blockNumber"}, float32(blockNumber.Int64()))
	}

	// Block

	block, err := m.ethClient.BlockByNumber(blockNumber)
	if err != nil {
		errors = multierror.Append(errors, err)
	} else {
		if m.lastBlock != nil {
			blockTime := block.Timestamp.Sub(*m.lastBlock.Timestamp)
			metrics.SetGauge([]string{"blocktime"}, float32(blockTime.Seconds()))
		}
		m.lastBlock = block
	}

	// Etherscan

	realBlockNumber, err := m.etherscan.BlockNumber()
	if err != nil {
		errors = multierror.Append(errors, err)
	} else {
		blocksbehind := Sub(realBlockNumber, blockNumber)
		metrics.SetGauge([]string{"blocksbehind"}, float32(blocksbehind.Int64()))

		if blocksbehind.Int64() == 0 {
			m.synced = true
		}
	}

	return errors
}
