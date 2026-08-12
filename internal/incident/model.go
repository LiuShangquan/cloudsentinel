package incident

import "time"

const (
	StatusFiring       = "firing"
	StatusAcknowledged = "acknowledged"
	StatusProcessing   = "processing"
	StatusResolved     = "resolved"
	StatusClosed       = "closed"
)

type Incident struct {
	ID                  uint64     `gorm:"primaryKey" json:"id"`
	EventKey            string     `gorm:"size:64;uniqueIndex" json:"event_key"`
	Fingerprint         string     `gorm:"size:64" json:"fingerprint"`
	ExternalFingerprint *string    `json:"external_fingerprint"`
	AlertName           string     `json:"alert_name"`
	ServiceID           uint64     `json:"service_id"`
	TaskID              uint64     `json:"task_id"`
	ProbeType           string     `json:"probe_type"`
	Severity            string     `json:"severity"`
	Status              string     `json:"status"`
	Summary             string     `json:"summary"`
	Description         string     `json:"description"`
	FiredAt             time.Time  `json:"fired_at"`
	LastSeenAt          time.Time  `json:"last_seen_at"`
	ResolvedAt          *time.Time `json:"resolved_at"`
	ClosedAt            *time.Time `json:"closed_at"`
	AcknowledgedAt      *time.Time `json:"acknowledged_at"`
	ProcessingAt        *time.Time `json:"processing_at"`
	AcknowledgedBy      *uint64    `json:"acknowledged_by"`
	ProcessingBy        *uint64    `json:"processing_by"`
	ClosedBy            *uint64    `json:"closed_by"`
	OccurrenceCount     uint64     `json:"occurrence_count"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

func (Incident) TableName() string { return "incidents" }

type Pagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int64 `json:"total_pages"`
}
type Page struct {
	Items      []Incident `json:"items"`
	Pagination Pagination `json:"pagination"`
}

type Webhook struct {
	Status string  `json:"status"`
	Alerts []Alert `json:"alerts"`
}
type Alert struct {
	Status      string            `json:"status"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	StartsAt    time.Time         `json:"startsAt"`
	EndsAt      time.Time         `json:"endsAt"`
	Fingerprint string            `json:"fingerprint"`
}
