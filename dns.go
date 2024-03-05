package main

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	outcomeDesc = prometheus.NewDesc(
		"simple_dns_exporter_outcome",
		`Query outcome`,
		[]string{"instance", "server", "outcome"},
		nil,
	)
	durationDesc = prometheus.NewDesc(
		"simple_dns_exporter_duration",
		`Duration in seconds for query response. Omitted if timeout or no response.`,
		[]string{"instance", "server"},
		nil,
	)
	answersDesc = prometheus.NewDesc(
		"simple_dns_exporter_answers_total",
		`Total number of answers. Omitted if timeout or no response.`,
		[]string{"instance", "server"},
		nil,
	)

	dnsDescriptions = []*prometheus.Desc{
		outcomeDesc,
		durationDesc,
		answersDesc,
	}
)

func probe(ctx context.Context, target, server string) (metrics []prometheus.Metric) {
	m := new(dns.Msg)
	m.Id = dns.Id()
	m.RecursionDesired = true
	m.Question = make([]dns.Question, 1)
	m.Question[0] = dns.Question{
		Name:   dns.Fqdn(target),
		Qtype:  dns.TypeA,
		Qclass: dns.ClassINET,
	}
	before := time.Now()
	r, err := dns.ExchangeContext(ctx, m, server)
	if err != nil {
		log.Printf("query err for %q on %q: %v", target, server, err)
	}
	if r == nil {
		r = &dns.Msg{}
	}
	dur := time.Since(before)
	// Each label combo is its own time-series in prom; we need to 0 out the other cases or prom
	// will just assume it wasn't collected and fill the space.
	metrics = append(metrics, prometheus.MustNewConstMetric(
		outcomeDesc,
		prometheus.GaugeValue,
		b2f(err == nil && r.Rcode == dns.RcodeSuccess),
		target,
		server,
		dns.RcodeToString[dns.RcodeSuccess]))
	metrics = append(metrics, prometheus.MustNewConstMetric(
		outcomeDesc,
		prometheus.GaugeValue,
		b2f(err == nil && r.Rcode == dns.RcodeNameError),
		target,
		server,
		dns.RcodeToString[dns.RcodeNameError]))
	metrics = append(metrics, prometheus.MustNewConstMetric(
		outcomeDesc,
		prometheus.GaugeValue,
		b2f(err == nil && r.Rcode == dns.RcodeServerFailure),
		target,
		server,
		dns.RcodeToString[dns.RcodeServerFailure]))
	metrics = append(metrics, prometheus.MustNewConstMetric(
		outcomeDesc,
		prometheus.GaugeValue,
		b2f(err == nil && r.Rcode != dns.RcodeSuccess && r.Rcode != dns.RcodeNameError && r.Rcode != dns.RcodeServerFailure),
		target,
		server,
		"other_rcode"))
	metrics = append(metrics, prometheus.MustNewConstMetric(
		outcomeDesc,
		prometheus.GaugeValue,
		b2f(errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)),
		target,
		server,
		"timeout"))
	metrics = append(metrics, prometheus.MustNewConstMetric(
		outcomeDesc,
		prometheus.GaugeValue,
		b2f(err != nil && !(errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled))),
		target,
		server,
		"unknown_error"))
	if err == nil {
		// We only send the duration/answer-count on success so we can calculate sensible averages.
		metrics = append(metrics, prometheus.MustNewConstMetric(
			durationDesc,
			prometheus.GaugeValue,
			dur.Seconds(),
			target,
			server))
		metrics = append(metrics, prometheus.MustNewConstMetric(
			answersDesc,
			prometheus.GaugeValue,
			float64(len(r.Answer)),
			target,
			server))
	}
	return metrics
}

func b2f(b bool) float64 {
	if b {
		return 1
	}
	return 0
}
