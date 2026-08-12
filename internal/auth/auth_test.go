package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"cloudsentinel/internal/audit"
	"cloudsentinel/internal/platform/config"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type memoryStore struct {
	users           map[string]User
	failures        []audit.Log
	successes       []audit.Log
	createCount     int
	createErr       error
	userOnCreateErr *User
	passwordAtStart string
}

func newMemoryStore(users ...User) *memoryStore {
	store := &memoryStore{users: map[string]User{}}
	for _, user := range users {
		store.users[user.Username] = user
	}
	return store
}

func (s *memoryStore) FindByUsername(_ context.Context, username string) (User, error) {
	user, ok := s.users[username]
	if !ok {
		return User{}, ErrUserNotFound
	}
	return user, nil
}

func (s *memoryStore) FindByID(_ context.Context, id uint64) (User, error) {
	for _, user := range s.users {
		if user.ID == id {
			return user, nil
		}
	}
	return User{}, ErrUserNotFound
}

func (s *memoryStore) CreateUser(_ context.Context, user *User) error {
	s.createCount++
	if s.createErr != nil {
		if s.userOnCreateErr != nil {
			s.users[s.userOnCreateErr.Username] = *s.userOnCreateErr
		}
		return s.createErr
	}
	user.ID = uint64(len(s.users) + 1)
	s.users[user.Username] = *user
	return nil
}

func (s *memoryStore) RecordFailure(_ context.Context, entry audit.Log) error {
	s.failures = append(s.failures, entry)
	return nil
}

func (s *memoryStore) RecordSuccess(_ context.Context, id uint64, at time.Time, entry audit.Log) error {
	s.successes = append(s.successes, entry)
	for name, user := range s.users {
		if user.ID == id {
			user.LastLoginAt = &at
			s.users[name] = user
		}
	}
	return nil
}

func TestTokenValidation(t *testing.T) {
	manager, err := NewTokenManager("correct-secret", "cloudsentinel-api", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := manager.Generate(User{ID: 7, Username: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	principal, err := manager.Parse(token)
	if err != nil || principal.UserID != 7 || principal.Username != "admin" {
		t.Fatalf("Parse() = %+v, %v", principal, err)
	}

	wrongSecret, _ := NewTokenManager("wrong-secret", "cloudsentinel-api", time.Hour)
	if _, err := wrongSecret.Parse(token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("wrong secret error = %v", err)
	}
	parts := strings.Split(token, ".")
	parts[2] = parts[2] + "x"
	if _, err := manager.Parse(strings.Join(parts, ".")); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("tampered token error = %v", err)
	}
}

func TestTokenRejectsExpiredAndNone(t *testing.T) {
	manager, _ := NewTokenManager("correct-secret", "cloudsentinel-api", time.Hour)
	manager.now = func() time.Time { return time.Now().Add(-2 * time.Hour) }
	expired, _, _ := manager.Generate(User{ID: 1, Username: "admin"})
	if _, err := manager.Parse(expired); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expired token error = %v", err)
	}

	claims := Claims{Username: "admin", RegisteredClaims: jwt.RegisteredClaims{Subject: "1", Issuer: "cloudsentinel-api", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}
	none, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Parse(none); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("alg none error = %v", err)
	}
}

func TestLoginAndMe(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	store := newMemoryStore(User{ID: 1, Username: "admin", PasswordHash: string(hash), Status: StatusActive})
	tokens, _ := NewTokenManager("secret", "issuer", time.Hour)
	service := NewService(store, tokens)

	result, err := service.Login(context.Background(), "admin", "password", RequestMetadata{RequestID: "request-1"})
	if err != nil || result.AccessToken == "" || result.User.Username != "admin" {
		t.Fatalf("Login() = %+v, %v", result, err)
	}
	if result.User.ID != 1 || len(store.successes) != 1 || result.User.LastLoginAt == nil {
		t.Fatalf("login did not persist safe result: %+v", result)
	}
	if _, err := service.Me(context.Background(), 1); err != nil {
		t.Fatalf("Me() error = %v", err)
	}
}

func TestLoginFailuresAreSafeAndUnified(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	store := newMemoryStore(
		User{ID: 1, Username: "admin", PasswordHash: string(hash), Status: StatusActive},
		User{ID: 2, Username: "disabled", PasswordHash: string(hash), Status: StatusDisabled},
	)
	tokens, _ := NewTokenManager("secret", "issuer", time.Hour)
	service := NewService(store, tokens)
	if _, err := service.Login(context.Background(), "admin", "wrong", RequestMetadata{}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password error = %v", err)
	}
	if _, err := service.Login(context.Background(), "missing", "wrong", RequestMetadata{}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("missing user error = %v", err)
	}
	if _, err := service.Login(context.Background(), "disabled", "password", RequestMetadata{}); !errors.Is(err, ErrUserDisabled) {
		t.Fatalf("disabled user error = %v", err)
	}
	if len(store.failures) != 3 {
		t.Fatalf("failure audits = %d", len(store.failures))
	}
}

func TestBootstrapIsValidatedAndIdempotent(t *testing.T) {
	store := newMemoryStore()
	if err := Bootstrap(context.Background(), store, config.BootstrapConfig{Username: "only"}); err == nil {
		t.Fatal("expected partial bootstrap configuration error")
	}
	cfg := config.BootstrapConfig{Username: "admin", Password: "password"}
	if err := Bootstrap(context.Background(), store, cfg); err != nil {
		t.Fatal(err)
	}
	original := store.users["admin"].PasswordHash
	if err := Bootstrap(context.Background(), store, config.BootstrapConfig{Username: "admin", Password: "different"}); err != nil {
		t.Fatal(err)
	}
	if store.createCount != 1 || store.users["admin"].PasswordHash != original {
		t.Fatal("bootstrap was not idempotent")
	}
}

func TestBootstrapAcceptsConcurrentCreateWinner(t *testing.T) {
	concurrent := User{ID: 9, Username: "admin", PasswordHash: "already-created", Status: StatusActive}
	store := newMemoryStore()
	store.createErr = errors.New("duplicate key")
	store.userOnCreateErr = &concurrent

	err := Bootstrap(context.Background(), store, config.BootstrapConfig{Username: "admin", Password: "password"})
	if err != nil {
		t.Fatalf("Bootstrap() concurrent winner error = %v", err)
	}
	if store.createCount != 1 || store.users["admin"].ID != concurrent.ID {
		t.Fatalf("concurrent bootstrap result = %+v", store.users["admin"])
	}
}
