package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"cloudsentinel/internal/audit"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrUserDisabled       = errors.New("user is disabled")
	ErrUnauthenticated    = errors.New("unauthenticated")
)

type RequestMetadata struct {
	RequestID string
	ClientIP  string
	UserAgent string
}

type LoginResult struct {
	AccessToken string     `json:"access_token"`
	TokenType   string     `json:"token_type"`
	ExpiresIn   int64      `json:"expires_in"`
	User        PublicUser `json:"user"`
}

type Service struct {
	store  Store
	tokens *TokenManager
	now    func() time.Time
}

func NewService(store Store, tokens *TokenManager) *Service {
	return &Service{store: store, tokens: tokens, now: time.Now}
}

func (s *Service) Login(ctx context.Context, username, password string, metadata RequestMetadata) (LoginResult, error) {
	username = strings.TrimSpace(username)
	user, err := s.store.FindByUsername(ctx, username)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		s.recordFailure(ctx, username, "invalid_credentials", metadata)
		return LoginResult{}, ErrInvalidCredentials
	}
	if user.Status != StatusActive {
		s.recordFailure(ctx, username, "user_disabled", metadata)
		return LoginResult{}, ErrUserDisabled
	}
	token, expiresAt, err := s.tokens.Generate(user)
	if err != nil {
		return LoginResult{}, err
	}
	now := s.now().UTC()
	entry := loginAudit(&user.ID, username, "success", "", metadata)
	if err := s.store.RecordSuccess(ctx, user.ID, now, entry); err != nil {
		return LoginResult{}, err
	}
	user.LastLoginAt = &now
	return LoginResult{AccessToken: token, TokenType: "Bearer", ExpiresIn: int64(time.Until(expiresAt).Seconds()), User: publicUser(user)}, nil
}

func (s *Service) Me(ctx context.Context, userID uint64) (PublicUser, error) {
	user, err := s.store.FindByID(ctx, userID)
	if err != nil || user.Status != StatusActive {
		return PublicUser{}, ErrUnauthenticated
	}
	return publicUser(user), nil
}

func (s *Service) recordFailure(ctx context.Context, username, code string, metadata RequestMetadata) {
	_ = s.store.RecordFailure(ctx, loginAudit(nil, username, "failure", code, metadata))
}

func loginAudit(userID *uint64, username, outcome, failure string, metadata RequestMetadata) audit.Log {
	action := "auth.login"
	entry := audit.Log{ActorUserID: userID, Username: pointer(username), Action: action, Outcome: outcome, RequestID: pointer(metadata.RequestID), ClientIP: pointer(metadata.ClientIP), UserAgent: pointer(metadata.UserAgent)}
	if failure != "" {
		entry.FailureCode = pointer(failure)
	}
	return entry
}

func pointer(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
