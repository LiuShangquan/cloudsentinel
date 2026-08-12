package auth

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var ErrInvalidToken = errors.New("invalid token")

type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

type Principal struct {
	UserID    uint64
	Username  string
	ExpiresAt time.Time
}

type TokenManager struct {
	secret    []byte
	issuer    string
	expiresIn time.Duration
	now       func() time.Time
}

func NewTokenManager(secret, issuer string, expiresIn time.Duration) (*TokenManager, error) {
	if secret == "" {
		return nil, errors.New("configuration JWT_SECRET: required")
	}
	if issuer == "" {
		return nil, errors.New("configuration JWT_ISSUER: required")
	}
	if expiresIn <= 0 {
		return nil, errors.New("configuration JWT_EXPIRES_IN: must be positive")
	}
	return &TokenManager{secret: []byte(secret), issuer: issuer, expiresIn: expiresIn, now: time.Now}, nil
}

func (m *TokenManager) Generate(user User) (string, time.Time, error) {
	now := m.now().UTC()
	expiresAt := now.Add(m.expiresIn)
	claims := Claims{
		Username: user.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatUint(user.ID, 10),
			Issuer:    m.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign JWT: %w", err)
	}
	return signed, expiresAt, nil
}

func (m *TokenManager) Parse(raw string) (Principal, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalidToken
		}
		return m.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer(m.issuer), jwt.WithExpirationRequired())
	if err != nil || !token.Valid || claims.ExpiresAt == nil {
		return Principal{}, ErrInvalidToken
	}
	userID, err := strconv.ParseUint(claims.Subject, 10, 64)
	if err != nil || userID == 0 {
		return Principal{}, ErrInvalidToken
	}
	return Principal{UserID: userID, Username: claims.Username, ExpiresAt: claims.ExpiresAt.Time}, nil
}
