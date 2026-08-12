package metrics

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"gorm.io/gorm"
)

type ActiveIncidentCollector struct {
	db   *gorm.DB
	desc *prometheus.Desc
}

func NewActiveIncidentCollector(db *gorm.DB) *ActiveIncidentCollector {
	return &ActiveIncidentCollector{db: db, desc: prometheus.NewDesc("cloudsentinel_active_incidents", "Active incidents from shared MySQL state.", nil, nil)}
}
func (c *ActiveIncidentCollector) Describe(channel chan<- *prometheus.Desc) { channel <- c.desc }
func (c *ActiveIncidentCollector) Collect(channel chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var count int64
	if err := c.db.WithContext(ctx).Table("incidents").Where("status IN ?", []string{"firing", "acknowledged", "processing"}).Count(&count).Error; err != nil {
		return
	}
	channel <- prometheus.MustNewConstMetric(c.desc, prometheus.GaugeValue, float64(count))
}
