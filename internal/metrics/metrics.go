package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const namespace = "entain"

// Collector exposes HTTP and business metrics for observability.
type Collector struct {
	registry          *prometheus.Registry
	requestsTotal     *prometheus.CounterVec
	requestDuration   *prometheus.HistogramVec
	txApplied         prometheus.Counter
	txRejected        *prometheus.CounterVec
	insufficientFunds prometheus.Counter
}

// New creates an isolated metrics collector with its own registry.
func New() *Collector {
	reg := prometheus.NewRegistry()

	requestsTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "http_requests_total",
		Help:      "Total number of HTTP requests.",
	}, []string{"method", "route", "status"})
	requestDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "http_request_duration_seconds",
		Help:      "HTTP request latency in seconds.",
		Buckets:   []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
	}, []string{"method", "route"})
	txApplied := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "transactions_applied_total",
		Help:      "Number of balance-changing transactions committed.",
	})
	txRejected := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "transactions_rejected_total",
		Help:      "Number of rejected transaction attempts.",
	}, []string{"reason"})
	insufficientFunds := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "insufficient_funds_total",
		Help:      "Number of transaction attempts rejected due to insufficient balance (HTTP 402).",
	})

	buildInfo := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "build_info",
		Help:      "Static build metadata for the balance service.",
	}, []string{"service", "go_version"})
	buildInfo.WithLabelValues("balance-api", "1.26").Set(1)

	reg.MustRegister(requestsTotal, requestDuration, txApplied, txRejected, insufficientFunds, buildInfo)

	// Expose zero-valued rejection series so PromQL ratios work before first error.
	for _, reason := range []string{
		"invalid_user_id",
		"missing_source_type",
		"invalid_json",
		"user_not_found",
		"duplicate_transaction",
		"insufficient_funds",
		"validation",
		"internal",
	} {
		txRejected.WithLabelValues(reason)
	}

	return &Collector{
		registry:          reg,
		requestsTotal:     requestsTotal,
		requestDuration:   requestDuration,
		txApplied:         txApplied,
		txRejected:        txRejected,
		insufficientFunds: insufficientFunds,
	}
}

// Handler returns the Prometheus scrape endpoint.
func (c *Collector) Handler() http.Handler {
	return promhttp.HandlerFor(c.registry, promhttp.HandlerOpts{})
}

// ObserveRequest records HTTP request metrics.
func (c *Collector) ObserveRequest(method, route string, status int, duration time.Duration) {
	c.requestsTotal.WithLabelValues(method, route, strconv.Itoa(status)).Inc()
	c.requestDuration.WithLabelValues(method, route).Observe(duration.Seconds())
}

// IncTxApplied increments successful transaction counter.
func (c *Collector) IncTxApplied() {
	c.txApplied.Inc()
}

// IncTxRejected increments rejected transaction counter by reason.
func (c *Collector) IncTxRejected(reason string) {
	c.txRejected.WithLabelValues(reason).Inc()
}

// IncInsufficientFunds increments insufficient balance rejections (HTTP 402).
func (c *Collector) IncInsufficientFunds() {
	c.insufficientFunds.Inc()
}
