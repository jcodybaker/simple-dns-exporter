package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	defaultBindAddr = "0.0.0.0:9153"
)

func main() {
	bindAddr := defaultBindAddr
	if b := os.Getenv("BIND_ADDR"); b != "" {
		bindAddr = b
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/probe", handleProbe)

	s := &http.Server{
		Addr:    bindAddr,
		Handler: mux,
	}
	log.Printf("starting server on %s", bindAddr)
	log.Fatal(s.ListenAndServe())
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	http.Error(w, http.StatusText(http.StatusOK), http.StatusOK)
}

func handleProbe(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		log.Default().Printf("err parsing form: %v", err)
		return
	}
	target := r.Form.Get("target")
	server := r.Form.Get("server")
	if target == "" {
		log.Default().Println("request had empty target")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	if server == "" {
		log.Default().Println("request had empty server")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	if !strings.ContainsRune(server, ':') {
		server = net.JoinHostPort(server, "53")
	}
	m := probe(r.Context(), target, server)
	pr := prometheus.NewRegistry()
	pr.MustRegister(&staticCollector{
		descs:   dnsDescriptions,
		metrics: m,
	})
	promhttp.HandlerFor(pr, promhttp.HandlerOpts{}).ServeHTTP(w, r)
}

// staticCollector is a lazy way to get metrics into wire-format.
type staticCollector struct {
	descs   []*prometheus.Desc
	metrics []prometheus.Metric
}

func (s *staticCollector) Describe(c chan<- *prometheus.Desc) {
	for _, d := range s.descs {
		c <- d
	}
}

func (s *staticCollector) Collect(c chan<- prometheus.Metric) {
	for _, m := range s.metrics {
		c <- m
	}
}
