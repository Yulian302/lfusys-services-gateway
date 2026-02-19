package auth

import (
	"context"

	logger "github.com/Yulian302/lfusys-services-commons/logging"
	"github.com/Yulian302/lfusys-services-gateway/store"
)

type RegisterStateManager struct {
	sessionStore store.SessionStore

	logger logger.Logger
}

func NewRegisterStateManager(store store.SessionStore, l logger.Logger) *RegisterStateManager {
	return &RegisterStateManager{
		sessionStore: store,
		logger:       l,
	}
}

func (s *RegisterStateManager) SaveState(ctx context.Context, state string) error {
	if err := s.sessionStore.Create(ctx, state); err != nil {
		s.logger.Warn("state store unavailable, continuing without persistence",
			"error", err,
		)
	}
	return nil
}

func (s *RegisterStateManager) IsValidState(ctx context.Context, callbackState string) (bool, error) {
	isStateExists, err := s.sessionStore.IsStateExists(ctx, callbackState)
	if err != nil {
		s.logger.Warn("state store unavailable, failing open",
			"error", err,
		)
		return true, nil
	}
	if !isStateExists {
		s.logger.Debug("state validation failed",
			"reason", "state_not_found",
		)
		return false, nil
	}
	s.logger.Debug("state validation succeeded")
	return true, nil
}
