package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type HttpServer struct {
	logger   *log.Logger
	monitor  *Monitor
	HTTPAddr net.Addr
	mux      *http.ServeMux
	listener net.Listener
}

func NewHttpServer(logger *log.Logger, monitor *Monitor, HTTPAddr net.Addr) *HttpServer {
	return &HttpServer{
		logger:   logger,
		monitor:  monitor,
		HTTPAddr: HTTPAddr,
	}
}

func (h *HttpServer) Start(ctx context.Context) error {

	l, err := net.Listen("tcp", h.HTTPAddr.String())
	if err != nil {
		return fmt.Errorf("failed to start listner on %s: %v", h.HTTPAddr.String(), err)
	}

	go func() {
		<-ctx.Done()
		h.logger.Printf("Shutting down http server")

		if err := l.Close(); err != nil {
			h.logger.Printf("Failed to close http server: %v", err)
		}
	}()

	h.listener = l

	h.mux = http.NewServeMux()
	h.mux.Handle("/metrics", h.wrap(h.MetricsRequest))

	go http.Serve(l, h.mux)

	h.logger.Printf("Http api running on %s", h.HTTPAddr.String())

	return nil
}

func (h *HttpServer) wrap(handler func(resp http.ResponseWriter, req *http.Request) (interface{}, error)) http.HandlerFunc {
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

func (h *HttpServer) MetricsRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "GET" {
		return nil, fmt.Errorf("Incorrect method. Found %s, only GET available", req.Method)
	}

	if format := req.URL.Query().Get("format"); format == "prometheus" {
		handler := promhttp.Handler()
		handler.ServeHTTP(resp, req)
		return nil, nil
	}

	return h.monitor.InmemSink.DisplayMetrics(resp, req)
}
