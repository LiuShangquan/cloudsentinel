package asset

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"cloudsentinel/internal/audit"
)

var ErrHostDisabled = errors.New("host is disabled")

type Metadata struct{ RequestID, ClientIP, UserAgent string }

type HostInput struct{ Name, Address, Description string }
type ServiceInput struct {
	HostID                          uint64
	Name, Type, Target, Description string
}

type AssetService struct{ store Store }

func NewService(store Store) *AssetService { return &AssetService{store: store} }

func (s *AssetService) CreateHost(ctx context.Context, input HostInput, actor uint64, metadata Metadata) (Host, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Address = strings.TrimSpace(input.Address)
	if input.Name == "" || len(input.Name) > 100 || validateHostAddress(input.Address) != nil {
		return Host{}, ErrInvalidInput
	}
	host := Host{Name: input.Name, Address: input.Address, Description: input.Description, Status: StatusActive, CreatedBy: actor, UpdatedBy: actor}
	if err := s.store.CreateHost(ctx, &host, auditEntry(actor, "host.create", "host", 0, metadata)); err != nil {
		return Host{}, err
	}
	return host, nil
}

func (s *AssetService) GetHost(ctx context.Context, id uint64) (Host, error) {
	return s.store.GetHost(ctx, id)
}
func (s *AssetService) ListHosts(ctx context.Context, page, size int) (Page[Host], error) {
	items, total, err := s.store.ListHosts(ctx, page, size)
	if err != nil {
		return Page[Host]{}, err
	}
	return Page[Host]{Items: items, Pagination: pagination(page, size, total)}, nil
}
func (s *AssetService) UpdateHost(ctx context.Context, id uint64, input HostInput, actor uint64, metadata Metadata) (Host, error) {
	if _, err := s.store.GetHost(ctx, id); err != nil {
		return Host{}, err
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Address = strings.TrimSpace(input.Address)
	if input.Name == "" || len(input.Name) > 100 || validateHostAddress(input.Address) != nil {
		return Host{}, ErrInvalidInput
	}
	host := Host{ID: id, Name: input.Name, Address: input.Address, Description: input.Description, UpdatedBy: actor}
	if err := s.store.UpdateHost(ctx, &host, auditEntry(actor, "host.update", "host", id, metadata)); err != nil {
		return Host{}, err
	}
	return s.store.GetHost(ctx, id)
}
func (s *AssetService) DisableHost(ctx context.Context, id, actor uint64, metadata Metadata) error {
	if _, err := s.store.GetHost(ctx, id); err != nil {
		return err
	}
	count, err := s.store.CountActiveServices(ctx, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrConflict
	}
	return s.store.DisableHost(ctx, id, actor, auditEntry(actor, "host.disable", "host", id, metadata))
}

func (s *AssetService) CreateMonitoredService(ctx context.Context, input ServiceInput, actor uint64, metadata Metadata) (MonitoredService, error) {
	host, err := s.store.GetHost(ctx, input.HostID)
	if err != nil {
		return MonitoredService{}, err
	}
	if host.Status != StatusActive {
		return MonitoredService{}, ErrHostDisabled
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Target = strings.TrimSpace(input.Target)
	if input.Name == "" || len(input.Name) > 100 || validateTarget(input.Type, input.Target) != nil {
		return MonitoredService{}, ErrInvalidInput
	}
	service := MonitoredService{HostID: input.HostID, Name: input.Name, Type: input.Type, Target: input.Target, Description: input.Description, Status: StatusActive, CreatedBy: actor, UpdatedBy: actor}
	if err := s.store.CreateService(ctx, &service, auditEntry(actor, "service.create", "service", 0, metadata)); err != nil {
		return MonitoredService{}, err
	}
	return service, nil
}
func (s *AssetService) GetMonitoredService(ctx context.Context, id uint64) (MonitoredService, error) {
	return s.store.GetService(ctx, id)
}
func (s *AssetService) ListMonitoredServices(ctx context.Context, page, size int) (Page[MonitoredService], error) {
	items, total, err := s.store.ListServices(ctx, page, size)
	if err != nil {
		return Page[MonitoredService]{}, err
	}
	return Page[MonitoredService]{Items: items, Pagination: pagination(page, size, total)}, nil
}
func (s *AssetService) UpdateMonitoredService(ctx context.Context, id uint64, input ServiceInput, actor uint64, metadata Metadata) (MonitoredService, error) {
	if _, err := s.store.GetService(ctx, id); err != nil {
		return MonitoredService{}, err
	}
	host, err := s.store.GetHost(ctx, input.HostID)
	if err != nil {
		return MonitoredService{}, err
	}
	if host.Status != StatusActive {
		return MonitoredService{}, ErrHostDisabled
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Target = strings.TrimSpace(input.Target)
	if input.Name == "" || len(input.Name) > 100 || validateTarget(input.Type, input.Target) != nil {
		return MonitoredService{}, ErrInvalidInput
	}
	service := MonitoredService{ID: id, HostID: input.HostID, Name: input.Name, Type: input.Type, Target: input.Target, Description: input.Description, UpdatedBy: actor}
	if err := s.store.UpdateService(ctx, &service, auditEntry(actor, "service.update", "service", id, metadata)); err != nil {
		return MonitoredService{}, err
	}
	return s.store.GetService(ctx, id)
}
func (s *AssetService) DisableMonitoredService(ctx context.Context, id, actor uint64, metadata Metadata) error {
	if _, err := s.store.GetService(ctx, id); err != nil {
		return err
	}
	count, err := s.store.CountActiveProbeTasks(ctx, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrConflict
	}
	return s.store.DisableService(ctx, id, actor, auditEntry(actor, "service.disable", "service", id, metadata))
}

func pagination(page, size int, total int64) Pagination {
	pages := int64(0)
	if total > 0 {
		pages = (total + int64(size) - 1) / int64(size)
	}
	return Pagination{Page: page, PageSize: size, Total: total, TotalPages: pages}
}
func auditEntry(actor uint64, action, resource string, id uint64, metadata Metadata) audit.Log {
	username := ""
	outcome := "success"
	resourceID := ""
	if id > 0 {
		resourceID = strconv.FormatUint(id, 10)
	}
	return audit.Log{ActorUserID: &actor, Username: nullable(username), Action: action, Outcome: outcome, ResourceType: nullable(resource), ResourceID: nullable(resourceID), RequestID: nullable(metadata.RequestID), ClientIP: nullable(metadata.ClientIP), UserAgent: nullable(metadata.UserAgent)}
}
func nullable(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
