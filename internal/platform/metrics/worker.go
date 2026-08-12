package metrics

import (
	"context"
	"strconv"
	"time"

	"cloudsentinel/internal/probe"
	"github.com/prometheus/client_golang/prometheus"
	"gorm.io/gorm"
)

type Worker struct {
	Registry *prometheus.Registry
	total    *prometheus.CounterVec
	duration *prometheus.HistogramVec
	active   prometheus.Gauge
	queue    prometheus.Gauge
}

func NewWorker(db *gorm.DB) *Worker {
	registry := prometheus.NewRegistry()
	registry.MustRegister(prometheus.NewGoCollector(), prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	total := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "cloudsentinel_probe_total", Help: "Completed probes."}, []string{"probe_type", "outcome"})
	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "cloudsentinel_probe_duration_seconds", Help: "Probe execution duration in seconds.", Buckets: prometheus.DefBuckets}, []string{"probe_type"})
	active := prometheus.NewGauge(prometheus.GaugeOpts{Name: "cloudsentinel_worker_active", Help: "Workers currently processing probe executions."})
	queue := prometheus.NewGauge(prometheus.GaugeOpts{Name: "cloudsentinel_worker_queue_length", Help: "Jobs currently buffered in this worker process."})
	registry.MustRegister(total, duration, active, queue, newLatestProbeCollector(db))
	return &Worker{Registry: registry, total: total, duration: duration, active: active, queue: queue}
}

func (m *Worker) ActiveInc()               { m.active.Inc() }
func (m *Worker) ActiveDec()               { m.active.Dec() }
func (m *Worker) SetQueueLength(value int) { m.queue.Set(float64(value)) }
func (m *Worker) Observe(work probe.ExecutionWork, result probe.Result, duration time.Duration) {
	outcome := "failure"
	if result.Success {
		outcome = "success"
	}
	m.total.WithLabelValues(work.Execution.ProbeType, outcome).Inc()
	m.duration.WithLabelValues(work.Execution.ProbeType).Observe(duration.Seconds())
}

type latestProbeCollector struct {
	db   *gorm.DB
	desc *prometheus.Desc
}

func newLatestProbeCollector(db *gorm.DB) *latestProbeCollector {
	return &latestProbeCollector{db: db, desc: prometheus.NewDesc("cloudsentinel_probe_status", "Latest durable probe status (1 success, 0 failure).", []string{"service_id", "task_id", "probe_type"}, nil)}
}
func (c *latestProbeCollector) Describe(channel chan<- *prometheus.Desc) { channel <- c.desc }
func (c *latestProbeCollector) Collect(channel chan<- prometheus.Metric) {
	type row struct {
		ServiceID uint64
		TaskID    uint64
		ProbeType string
		Success   bool
	}
	var rows []row
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	query := `SELECT pr.service_id, pr.task_id, pr.probe_type, pr.success FROM probe_results pr JOIN (SELECT task_id, MAX(id) AS id FROM probe_results WHERE status IN ('succeeded','failed') GROUP BY task_id) latest ON latest.id = pr.id`
	if err := c.db.WithContext(ctx).Raw(query).Scan(&rows).Error; err != nil {
		return
	}
	for _, row := range rows {
		value := 0.0
		if row.Success {
			value = 1
		}
		channel <- prometheus.MustNewConstMetric(c.desc, prometheus.GaugeValue, value, strconv.FormatUint(row.ServiceID, 10), strconv.FormatUint(row.TaskID, 10), row.ProbeType)
	}
}
