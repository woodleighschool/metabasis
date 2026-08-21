package metrics

import (
	"context"
	"log/slog"
	"net/http"
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/woodleighschool/metabasis/internal/store"
)

const stateCollectionTimeout = 2 * time.Second

// WebhookResult is a bounded webhook request outcome label.
type WebhookResult string

const (
	WebhookAccepted     WebhookResult = "accepted"
	WebhookUnauthorized WebhookResult = "unauthorized"
	WebhookInvalid      WebhookResult = "invalid"
	WebhookError        WebhookResult = "error"
)

// BuildInfo identifies the running Metabasis binary.
type BuildInfo struct {
	Version  string
	Revision string
}

// Recorder owns the process-local Prometheus registry and event metrics.
type Recorder struct {
	registry               *prometheus.Registry
	logger                 *slog.Logger
	webhookRequests        *prometheus.CounterVec
	reconciliations        *prometheus.CounterVec
	reconciliationDuration *prometheus.HistogramVec
}

// New creates an isolated registry with standard runtime and Metabasis metrics.
func New(build BuildInfo, logger *slog.Logger) *Recorder {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	buildInfo := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "metabasis_build_info",
		Help: "Build information for the running Metabasis process.",
	}, []string{"version", "revision", "goversion"})
	buildInfo.WithLabelValues(build.Version, build.Revision, runtime.Version()).Set(1)

	webhookRequests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "metabasis_webhook_requests_total",
		Help: "Webhook requests processed by source and bounded result.",
	}, []string{"source", "result"})
	reconciliations := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "metabasis_reconciliations_total",
		Help: "Subject reconciliation attempts by result.",
	}, []string{"result"})
	reconciliationDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "metabasis_reconciliation_duration_seconds",
		Help: "Duration of subject reconciliation attempts by result.",
	}, []string{"result"})
	registry.MustRegister(buildInfo, webhookRequests, reconciliations, reconciliationDuration)
	return &Recorder{
		registry:               registry,
		logger:                 logger,
		webhookRequests:        webhookRequests,
		reconciliations:        reconciliations,
		reconciliationDuration: reconciliationDuration,
	}
}

// RegisterState adds the database-derived collector after the store is open.
func (r *Recorder) RegisterState(state *store.Store) {
	if r != nil && state != nil {
		r.registry.MustRegister(newStateCollector(state, r.logger))
	}
}

// Handler serves this recorder's registry in Prometheus exposition format.
func (r *Recorder) Handler() http.Handler {
	return promhttp.HandlerFor(r.registry, promhttp.HandlerOpts{})
}

// RecordWebhook increments one bounded webhook outcome.
func (r *Recorder) RecordWebhook(source string, result WebhookResult) {
	if r != nil {
		r.webhookRequests.WithLabelValues(source, string(result)).Inc()
	}
}

// RecordReconciliation records one subject reconciliation attempt.
func (r *Recorder) RecordReconciliation(err error, duration time.Duration) {
	if r == nil {
		return
	}
	result := "success"
	if err != nil {
		result = "error"
	}
	r.reconciliations.WithLabelValues(result).Inc()
	r.reconciliationDuration.WithLabelValues(result).Observe(duration.Seconds())
}

type stateReader interface {
	CollectMetrics(context.Context, time.Time) (store.MetricsSnapshot, error)
}

type stateCollector struct {
	state             stateReader
	logger            *slog.Logger
	intentDesc        *prometheus.Desc
	failedSubjectDesc *prometheus.Desc
	dueSubjectDesc    *prometheus.Desc
	successDesc       *prometheus.Desc
}

func newStateCollector(state stateReader, logger *slog.Logger) *stateCollector {
	return &stateCollector{
		state:  state,
		logger: logger,
		intentDesc: prometheus.NewDesc(
			"metabasis_intents",
			"Current persisted intents by temporal phase.",
			[]string{"phase"}, nil,
		),
		failedSubjectDesc: prometheus.NewDesc(
			"metabasis_reconciliation_failed_subjects",
			"Subjects whose most recent reconciliation failed.",
			nil, nil,
		),
		dueSubjectDesc: prometheus.NewDesc(
			"metabasis_reconciliation_due_subjects",
			"Subjects currently due for reconciliation.",
			nil, nil,
		),
		successDesc: prometheus.NewDesc(
			"metabasis_state_collection_success",
			"Whether the latest scrape collected persisted Metabasis state.",
			nil, nil,
		),
	}
}

func (c *stateCollector) Describe(descriptions chan<- *prometheus.Desc) {
	descriptions <- c.intentDesc
	descriptions <- c.failedSubjectDesc
	descriptions <- c.dueSubjectDesc
	descriptions <- c.successDesc
}

func (c *stateCollector) Collect(values chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), stateCollectionTimeout)
	defer cancel()
	snapshot, err := c.state.CollectMetrics(ctx, time.Now().UTC())
	if err != nil {
		c.logger.Error("collect Prometheus state", "error", err)
		values <- prometheus.MustNewConstMetric(c.successDesc, prometheus.GaugeValue, 0)
		return
	}
	values <- prometheus.MustNewConstMetric(c.successDesc, prometheus.GaugeValue, 1)
	for phase, count := range map[string]int64{
		"pending":   snapshot.PendingIntentCount,
		"active":    snapshot.ActiveIntentCount,
		"ended":     snapshot.EndedIntentCount,
		"cancelled": snapshot.CancelledIntentCount,
	} {
		values <- prometheus.MustNewConstMetric(c.intentDesc, prometheus.GaugeValue, float64(count), phase)
	}
	values <- prometheus.MustNewConstMetric(c.failedSubjectDesc, prometheus.GaugeValue, float64(snapshot.FailedSubjectCount))
	values <- prometheus.MustNewConstMetric(c.dueSubjectDesc, prometheus.GaugeValue, float64(snapshot.DueSubjectCount))
}
