package incident

import (
	"cloudsentinel/internal/audit"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Service struct {
	repo *Repository
	now  func() time.Time
}

func NewService(repo *Repository) *Service { return &Service{repo: repo, now: time.Now} }
func (s *Service) Ingest(ctx context.Context, payload Webhook) error {
	for _, alert := range payload.Alerts {
		if alert.Labels["cloudsentinel_incident"] != "true" {
			continue
		}
		status := alert.Status
		if status == "" {
			status = payload.Status
		}
		value, err := normalize(alert, s.now().UTC())
		if err != nil {
			return err
		}
		switch status {
		case StatusFiring:
			if err := s.repo.UpsertFiring(ctx, value); err != nil {
				return err
			}
		case StatusResolved:
			resolved := value.LastSeenAt
			value.Status = StatusResolved
			value.ResolvedAt = &resolved
			if err := s.repo.Resolve(ctx, value); err != nil {
				return err
			}
		default:
			return errors.New("unsupported alert status")
		}
	}
	return nil
}
func (s *Service) Get(ctx context.Context, id uint64) (Incident, error) { return s.repo.Get(ctx, id) }
func (s *Service) List(ctx context.Context, page, size int) (Page, error) {
	items, total, err := s.repo.List(ctx, page, size)
	if err != nil {
		return Page{}, err
	}
	pages := int64(0)
	if total > 0 {
		pages = (total + int64(size) - 1) / int64(size)
	}
	return Page{Items: items, Pagination: Pagination{Page: page, PageSize: size, Total: total, TotalPages: pages}}, nil
}
func (s *Service) Acknowledge(ctx context.Context, id, actor uint64, username string) (Incident, error) {
	return s.transition(ctx, id, actor, username, StatusAcknowledged, []string{StatusFiring}, "incident.acknowledge")
}
func (s *Service) Process(ctx context.Context, id, actor uint64, username string) (Incident, error) {
	return s.transition(ctx, id, actor, username, StatusProcessing, []string{StatusFiring, StatusAcknowledged}, "incident.process")
}
func (s *Service) Close(ctx context.Context, id, actor uint64, username string) (Incident, error) {
	return s.transition(ctx, id, actor, username, StatusClosed, []string{StatusResolved}, "incident.close")
}
func (s *Service) transition(ctx context.Context, id, actor uint64, username, to string, from []string, action string) (Incident, error) {
	if _, err := s.repo.Get(ctx, id); err != nil {
		return Incident{}, err
	}
	resource := "incident"
	resourceID := strconv.FormatUint(id, 10)
	outcome := "success"
	entry := audit.Log{ActorUserID: &actor, Username: &username, Action: action, Outcome: outcome, ResourceType: &resource, ResourceID: &resourceID}
	return s.repo.Transition(ctx, id, from, to, actor, entry)
}
func normalize(alert Alert, now time.Time) (Incident, error) {
	alertName := strings.TrimSpace(alert.Labels["alertname"])
	serviceID, err := strconv.ParseUint(alert.Labels["service_id"], 10, 64)
	if err != nil || serviceID == 0 {
		return Incident{}, errors.New("invalid service_id")
	}
	taskID, err := strconv.ParseUint(alert.Labels["task_id"], 10, 64)
	if err != nil || taskID == 0 {
		return Incident{}, errors.New("invalid task_id")
	}
	probeType := strings.TrimSpace(alert.Labels["probe_type"])
	if alertName == "" || probeType == "" || alert.StartsAt.IsZero() {
		return Incident{}, errors.New("missing stable alert fields")
	}
	canonical := fmt.Sprintf("alertname=%s\nservice_id=%d\ntask_id=%d\nprobe_type=%s", strings.ToLower(alertName), serviceID, taskID, strings.ToLower(probeType))
	fingerprint := sum(canonical)
	eventKey := sum(fingerprint + "\n" + alert.StartsAt.UTC().Format(time.RFC3339Nano))
	lastSeen := now
	if !alert.EndsAt.IsZero() {
		lastSeen = alert.EndsAt.UTC()
	}
	external := strings.TrimSpace(alert.Fingerprint)
	var externalPointer *string
	if external != "" {
		externalPointer = &external
	}
	return Incident{EventKey: eventKey, Fingerprint: fingerprint, ExternalFingerprint: externalPointer, AlertName: alertName, ServiceID: serviceID, TaskID: taskID, ProbeType: probeType, Severity: alert.Labels["severity"], Status: StatusFiring, Summary: alert.Annotations["summary"], Description: alert.Annotations["description"], FiredAt: alert.StartsAt.UTC(), LastSeenAt: lastSeen, OccurrenceCount: 1}, nil
}
func sum(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}
