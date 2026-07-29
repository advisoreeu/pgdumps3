// Package metrics exposes Prometheus metrics for pgdumps3 over an HTTP endpoint.
package metrics

import (
	"net/http"
	"runtime"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
)

const serverReadHeaderTimeout = 5 * time.Second

// Backup duration histogram buckets: 1s, 2s, 4s ... ~68min, covering small to
// very large dumps.
const (
	durationBucketStart  = 1.0
	durationBucketFactor = 2.0
	durationBucketCount  = 13
)

// Metrics holds the Prometheus collectors for pgdumps3 and their registry.
type Metrics struct {
	registry *prometheus.Registry

	runsTotal        *prometheus.CounterVec
	lastBackupSize   prometheus.Gauge
	lastSuccessTime  prometheus.Gauge
	backupDuration   prometheus.Histogram
	backupInProgress prometheus.Gauge
	buildInfo        *prometheus.GaugeVec
}

// New creates a Metrics instance with all collectors registered against a
// dedicated registry. The database name is attached to every backup metric as a
// constant label so multiple instances aggregate cleanly in one Prometheus.
func New(database string) *Metrics {
	labels := prometheus.Labels{"database": database}
	reg := prometheus.NewRegistry()

	m := &Metrics{
		registry: reg,
		runsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name:        "pgdumps3_backup_runs_total",
			Help:        "Total number of backup runs by outcome.",
			ConstLabels: labels,
		}, []string{"status"}),
		lastBackupSize: prometheus.NewGauge(prometheus.GaugeOpts{
			Name:        "pgdumps3_last_backup_size_bytes",
			Help:        "Compressed size in bytes of the most recent successful backup.",
			ConstLabels: labels,
		}),
		lastSuccessTime: prometheus.NewGauge(prometheus.GaugeOpts{
			Name:        "pgdumps3_last_success_timestamp_seconds",
			Help:        "Unix timestamp of the most recent successful backup.",
			ConstLabels: labels,
		}),
		backupDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:        "pgdumps3_backup_duration_seconds",
			Help:        "Duration of backup runs in seconds.",
			ConstLabels: labels,
			Buckets:     prometheus.ExponentialBuckets(durationBucketStart, durationBucketFactor, durationBucketCount),
		}),
		backupInProgress: prometheus.NewGauge(prometheus.GaugeOpts{
			Name:        "pgdumps3_backup_in_progress",
			Help:        "Whether a backup is currently running (1) or not (0).",
			ConstLabels: labels,
		}),
		buildInfo: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name:        "pgdumps3_build_info",
			Help:        "Build information for pgdumps3; the value is always 1.",
			ConstLabels: labels,
		}, []string{"version", "pg_version"}),
	}

	reg.MustRegister(
		m.runsTotal,
		m.lastBackupSize,
		m.lastSuccessTime,
		m.backupDuration,
		m.backupInProgress,
		m.buildInfo,
		// Only go_goroutines is kept from the Go runtime (catches a leaked
		// pg_dump goroutine); the full NewGoCollector memstats set is omitted.
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "go_goroutines",
			Help: "Number of goroutines that currently exist.",
		}, func() float64 { return float64(runtime.NumGoroutine()) }),
		// ProcessCollector is the reliable source for start-time/RSS/fds; the
		// series we don't want (cpu, network, virtual memory) are dropped at
		// exposition time via excludedDefaultMetrics.
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	// Initialise both outcome series so they are exported as 0 before the first
	// run, which makes rate() and alerting behave from the start.
	m.runsTotal.WithLabelValues("success")
	m.runsTotal.WithLabelValues("failure")

	return m
}

// RecordSuccess records a successful backup of the given compressed size and
// wall-clock duration.
func (m *Metrics) RecordSuccess(sizeBytes int64, d time.Duration) {
	m.runsTotal.WithLabelValues("success").Inc()
	m.lastBackupSize.Set(float64(sizeBytes))
	m.lastSuccessTime.SetToCurrentTime()
	m.backupDuration.Observe(d.Seconds())
}

// RecordFailure records a failed backup run of the given duration.
func (m *Metrics) RecordFailure(d time.Duration) {
	m.runsTotal.WithLabelValues("failure").Inc()
	m.backupDuration.Observe(d.Seconds())
}

// SetInProgress marks whether a backup is currently running.
func (m *Metrics) SetInProgress(inProgress bool) {
	if inProgress {
		m.backupInProgress.Set(1)

		return
	}

	m.backupInProgress.Set(0)
}

// SetBuildInfo records static build information as a constant metric.
func (m *Metrics) SetBuildInfo(version string, pgVersion int) {
	m.buildInfo.WithLabelValues(version, strconv.Itoa(pgVersion)).Set(1)
}

// excludedDefaultMetrics are ProcessCollector series we deliberately do not
// expose, to keep the scrape output minimal. Only start-time, resident memory
// and open/max file descriptors are kept.
var excludedDefaultMetrics = map[string]struct{}{
	"process_cpu_seconds_total":            {},
	"process_network_receive_bytes_total":  {},
	"process_network_transmit_bytes_total": {},
	"process_virtual_memory_bytes":         {},
	"process_virtual_memory_max_bytes":     {},
}

// filteredGatherer wraps a Gatherer and omits metric families whose name is in
// the exclude set. New pgdumps3 metrics pass through automatically.
type filteredGatherer struct {
	inner   prometheus.Gatherer
	exclude map[string]struct{}
}

func (f filteredGatherer) Gather() ([]*dto.MetricFamily, error) {
	mfs, err := f.inner.Gather()

	kept := mfs[:0]
	for _, mf := range mfs {
		if _, drop := f.exclude[mf.GetName()]; drop {
			continue
		}

		kept = append(kept, mf)
	}

	return kept, err
}

// NewServer builds an HTTP server that serves the metrics on /metrics at addr.
func (m *Metrics) NewServer(addr string) *http.Server {
	gatherer := filteredGatherer{inner: m.registry, exclude: excludedDefaultMetrics}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{}))

	return &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: serverReadHeaderTimeout,
	}
}
