package auth

import (
	"context"
	"errors"
	"fmt"

	"cloudsentinel/internal/platform/config"
	"golang.org/x/crypto/bcrypt"
)

func Bootstrap(ctx context.Context, store Store, cfg config.BootstrapConfig) error {
	if cfg.Username == "" && cfg.Password == "" {
		return nil
	}
	if cfg.Username == "" || cfg.Password == "" {
		return errors.New("configuration BOOTSTRAP_USER_USERNAME and BOOTSTRAP_USER_PASSWORD must be set together")
	}
	if _, err := store.FindByUsername(ctx, cfg.Username); err == nil {
		return nil
	} else if !errors.Is(err, ErrUserNotFound) {
		return fmt.Errorf("find bootstrap user: %w", err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash bootstrap password: %w", err)
	}
	user := &User{Username: cfg.Username, PasswordHash: string(hash), Status: StatusActive}
	if err := store.CreateUser(ctx, user); err != nil {
		// Multiple API replicas may bootstrap at the same time. If another
		// replica won the unique-key race, the desired user now exists and the
		// bootstrap operation is complete. Re-read instead of coupling this
		// domain operation to a database-driver-specific duplicate-key error.
		if _, findErr := store.FindByUsername(ctx, cfg.Username); findErr == nil {
			return nil
		}
		return fmt.Errorf("create bootstrap user: %w", err)
	}
	return nil
}
