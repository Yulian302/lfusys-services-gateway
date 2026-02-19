package auth

import (
	"context"
	cerr "errors"
	"fmt"
	"time"

	"github.com/Yulian302/lfusys-services-commons/caching"
	"github.com/Yulian302/lfusys-services-commons/errors"
	jwttypes "github.com/Yulian302/lfusys-services-commons/jwt"
	logger "github.com/Yulian302/lfusys-services-commons/logging"
	"github.com/Yulian302/lfusys-services-gateway/auth/oauth"
	"github.com/Yulian302/lfusys-services-gateway/auth/types"
	"github.com/Yulian302/lfusys-services-gateway/store"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type OAuthServiceDeps struct {
	UserStore     store.UserStore
	Cache         caching.CachingService
	AccessSecret  string
	RefreshSecret string

	Logger logger.Logger
}

type OAuthServiceImpl struct {
	userStore        store.UserStore
	cachingSvc       caching.CachingService
	JwtAccessSecret  string
	JwtRefreshSecret string

	logger logger.Logger
}

func NewOAuthServiceImpl(deps OAuthServiceDeps) *OAuthServiceImpl {
	return &OAuthServiceImpl{
		userStore:        deps.UserStore,
		cachingSvc:       deps.Cache,
		JwtAccessSecret:  deps.AccessSecret,
		JwtRefreshSecret: deps.RefreshSecret,
		logger:           deps.Logger,
	}
}

func (s *OAuthServiceImpl) LoginOAuth(ctx context.Context, email string) (*LoginResponse, error) {
	user, err := s.userStore.GetByEmail(ctx, email)
	if err != nil {
		s.logger.Error("oauth login failed",
			"email", email,
			"error", err,
		)
		return nil, err
	}

	tokenPair, err := s.generateTokenPair(user, s.JwtAccessSecret, s.JwtRefreshSecret)
	if err != nil {
		return nil, fmt.Errorf("generating token pair: %w", err)
	}

	s.logger.Info("user logged in via oauth",
		"email", email,
	)

	return &LoginResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		User:         user,
	}, nil
}

func (s *OAuthServiceImpl) RegisterOAuth(ctx context.Context, userData oauth.OAuthUser) (types.User, error) {
	user := newUserFromOAuth(userData)

	err := s.userStore.Create(ctx, user)
	if err != nil {
		if cerr.Is(err, errors.ErrUserAlreadyExists) {
			s.logger.Warn("oauth registration failed",
				"email", userData.Email,
				"reason", "user_already_exists",
			)
			return types.User{}, errors.ErrUserAlreadyExists
		}
		s.logger.Error("oauth registration failed",
			"email", userData.Email,
			"error", err,
		)
		return types.User{}, fmt.Errorf("db create user: %w", err)
	}

	s.logger.Info("user registered via oauth",
		"email", userData.Email,
		"provider", userData.Provider,
	)
	return user, nil
}
func (s *OAuthServiceImpl) generateTokenPair(user *types.User, accessSecret, refreshSecret string) (*jwttypes.TokenPair, error) {
	accessJti := uuid.New().String()
	accessClaims := jwttypes.JWTClaims{
		Issuer:    "lfusys",
		Subject:   user.Email,
		ExpiresAt: time.Now().Add(jwttypes.AccessTokenDuration).Unix(),
		IssuedAt:  time.Now().Unix(),
		Type:      "access",
		JTI:       accessJti,
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)

	accessToken, err := t.SignedString([]byte(accessSecret))
	if err != nil {
		s.logger.Error("could not sign JWT token",
			"email", user.Email,
			"error", err,
		)
		return nil, fmt.Errorf("%w: %w", errors.ErrTokenSignature, err)
	}

	refreshJti := uuid.New().String()
	refreshClaims := jwttypes.JWTClaims{
		Issuer:    "lfusys",
		Subject:   user.Email,
		ExpiresAt: time.Now().Add(jwttypes.RefreshTokenDuration).Unix(),
		IssuedAt:  time.Now().Unix(),
		Type:      "refresh",
		JTI:       refreshJti,
	}

	ref := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refs, err := ref.SignedString([]byte(refreshSecret))
	if err != nil {
		s.logger.Error("could not sign refresh token",
			"email", user.Email,
			"error", err,
		)
		return nil, fmt.Errorf("%w: %w", errors.ErrTokenSignature, err)
	}

	return &jwttypes.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refs,
	}, nil
}
func newUserFromOAuth(ouser oauth.OAuthUser) types.User {
	return types.User{
		ID: uuid.NewString(),
		RegisterUser: types.RegisterUser{
			Name:  ouser.Name,
			Email: ouser.Email,
		},
		OAuthProvider: ouser.Provider,
		OAuthID:       ouser.ProviderID,
		Verified:      true,
	}
}
