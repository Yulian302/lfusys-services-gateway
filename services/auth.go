package services

import (
	"context"
	"encoding/json"
	cerr "errors"
	"fmt"
	"time"

	"github.com/Yulian302/lfusys-services-commons/caching"
	logger "github.com/Yulian302/lfusys-services-commons/logging"
	"github.com/Yulian302/lfusys-services-commons/crypt"
	"github.com/Yulian302/lfusys-services-commons/errors"
	jwttypes "github.com/Yulian302/lfusys-services-commons/jwt"
	"github.com/Yulian302/lfusys-services-gateway/auth/oauth"
	"github.com/Yulian302/lfusys-services-gateway/auth/types"
	"github.com/Yulian302/lfusys-services-gateway/store"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type LoginResponse struct {
	AccessToken  string
	RefreshToken string
	User         *types.User
}

type JwtAuth interface {
	Login(ctx context.Context, email string, password string) (*LoginResponse, error)
	Register(ctx context.Context, req types.RegisterUser) error
	GetCurrentUser(ctx context.Context, accessToken string) (*types.User, error)
	RefreshToken(ctx context.Context, refreshToken string) (*jwttypes.TokenPair, error)
	SaveState(ctx context.Context, state string) error
}

type OAuth interface {
	LoginOAuth(ctx context.Context, email string) (*LoginResponse, error)
	RegisterOAuth(ctx context.Context, userData oauth.OAuthUser) (types.User, error)
	SaveState(ctx context.Context, state string) error
	IsValidState(ctx context.Context, callbackState string) (bool, error)
}

type AuthService interface {
	JwtAuth
	OAuth
}

type AuthServiceImpl struct {
	userStore        store.UserStore
	sessionStore     store.SessionStore
	cachingSvc       caching.CachingService
	JwtAccessSecret  string
	JwtRefreshSecret string

	logger logger.Logger
}

func NewAuthServiceImpl(userStore store.UserStore, sessionStore store.SessionStore, cachingSvc caching.CachingService, jwtAccessSecret, jwtRefreshSecret string, l logger.Logger) *AuthServiceImpl {
	return &AuthServiceImpl{
		userStore:        userStore,
		sessionStore:     sessionStore,
		cachingSvc:       cachingSvc,
		JwtAccessSecret:  jwtAccessSecret,
		JwtRefreshSecret: jwtRefreshSecret,
		logger:           l,
	}
}

func (s *AuthServiceImpl) GenerateTokenPair(user *types.User, accessSecret, refreshSecret string) (*jwttypes.TokenPair, error) {
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

func (s *AuthServiceImpl) Login(ctx context.Context, email string, password string) (*LoginResponse, error) {
	user, err := s.userStore.GetByEmail(ctx, email)
	if err != nil {
		s.logger.Warn("login failed",
			"email", email,
			"reason", "user_not_found",
		)
		return nil, fmt.Errorf("%w: %w", errors.ErrInvalidCredentials, err)
	}

	if !crypt.VerifyPasswordWithSalt(password, user.Password, user.Salt) {
		s.logger.Warn("login failed",
			"email", email,
			"reason", "invalid_password",
		)
		return nil, errors.ErrInvalidCredentials
	}

	tokenPair, err := s.GenerateTokenPair(user, s.JwtAccessSecret, s.JwtRefreshSecret)
	if err != nil {
		return nil, err
	}

	s.logger.Info("user logged in",
		"email", email,
	)

	return &LoginResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		User:         user,
	}, nil
}

func (s *AuthServiceImpl) LoginOAuth(ctx context.Context, email string) (*LoginResponse, error) {
	user, err := s.userStore.GetByEmail(ctx, email)
	if err != nil {
		s.logger.Error("oauth login failed",
			"email", email,
			"error", err,
		)
		return nil, err
	}

	tokenPair, err := s.GenerateTokenPair(user, s.JwtAccessSecret, s.JwtRefreshSecret)
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

func (s *AuthServiceImpl) Register(ctx context.Context, req types.RegisterUser) error {
	user := newUserFromRegistration(req)

	err := s.userStore.Create(ctx, user)
	if err != nil {
		if cerr.Is(err, errors.ErrUserAlreadyExists) {
			s.logger.Warn("user registration failed",
				"email", req.Email,
				"reason", "user_already_exists",
			)
			return fmt.Errorf("%w: %w", errors.ErrUserAlreadyExists, err)
		} else {
			s.logger.Error("user registration failed",
				"email", req.Email,
				"error", err,
			)
			return fmt.Errorf("%w: %w", errors.ErrInternalServer, err)
		}
	}

	s.logger.Info("user registered",
		"email", req.Email,
	)
	return nil
}

func (s *AuthServiceImpl) RegisterOAuth(ctx context.Context, userData oauth.OAuthUser) (types.User, error) {
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

func (s *AuthServiceImpl) GetCurrentUser(ctx context.Context, accessToken string) (*types.User, error) {

	claims, err := s.ValidateToken(accessToken)
	if err != nil {
		s.logger.Debug("get current user failed",
			"reason", "invalid_token",
		)
		return nil, fmt.Errorf("%w: %w", errors.ErrInvalidToken, err)
	}

	userKey := fmt.Sprintf("user:%s", claims.Subject)
	cached, err := s.cachingSvc.Get(ctx, userKey)
	if err == nil && cached != "" {
		var cachedUser types.User
		if err = json.Unmarshal([]byte(cached), &cachedUser); err == nil {
			s.logger.Debug("user retrieved from cache",
				"email", claims.Subject,
			)
			return &cachedUser, nil
		}
		s.logger.Debug("could not unmarshal cached user data",
			"email", claims.Subject,
		)
	}

	user, err := s.userStore.GetByEmail(ctx, claims.Subject)
	if err != nil {
		s.logger.Error("get current user failed",
			"email", claims.Subject,
			"reason", "user_not_found",
		)
		return nil, fmt.Errorf("%w: %w", errors.ErrUserNotFound, err)
	}

	b, err := json.Marshal(&types.PublicUser{
		ID:    user.ID,
		Email: user.Email,
		Name:  user.Name,
	})
	if err == nil {
		if err = s.cachingSvc.Set(ctx, userKey, string(b), 30*time.Minute); err != nil {
			s.logger.Debug("could not save user data in cache",
				"email", claims.Subject,
				"error", err,
			)
		}
	} else {
		s.logger.Debug("could not save user data in cache",
			"email", claims.Subject,
			"error", err,
		)
	}

	s.logger.Debug("user retrieved successfully",
		"email", claims.Subject,
	)
	return user, nil
}

func (s *AuthServiceImpl) ValidateToken(tokenString string) (*jwttypes.JWTClaims, error) {
	parsedToken, err := jwt.ParseWithClaims(tokenString, &jwttypes.JWTClaims{}, func(t *jwt.Token) (any, error) {
		return []byte(s.JwtAccessSecret), nil
	})

	if err != nil || !parsedToken.Valid {
		return nil, fmt.Errorf("%w: %w", errors.ErrInvalidToken, err)
	}

	return parsedToken.Claims.(*jwttypes.JWTClaims), nil
}

func (s *AuthServiceImpl) ValidateRefreshToken(tokenString string) (*jwttypes.JWTClaims, error) {
	parsedToken, err := jwt.ParseWithClaims(tokenString, &jwttypes.JWTClaims{}, func(t *jwt.Token) (any, error) {
		return []byte(s.JwtRefreshSecret), nil
	})

	if err != nil || !parsedToken.Valid {
		return nil, fmt.Errorf("%w: %w", errors.ErrInvalidToken, err)
	}

	return parsedToken.Claims.(*jwttypes.JWTClaims), nil
}

func (s *AuthServiceImpl) RefreshToken(ctx context.Context, refreshToken string) (*jwttypes.TokenPair, error) {
	claims, err := s.ValidateRefreshToken(refreshToken)
	if err != nil {
		s.logger.Debug("refresh token failed",
			"reason", "invalid_token",
		)
		return nil, fmt.Errorf("%w: %w", errors.ErrInvalidToken, err)
	}

	if claims.Type != "refresh" {
		s.logger.Debug("refresh token failed",
			"email", claims.Subject,
			"reason", "invalid_token_type",
		)
		return nil, fmt.Errorf("%w: %w", errors.ErrInvalidTokenType, err)
	}

	user, err := s.userStore.GetByEmail(ctx, claims.Subject)
	if err != nil || user == nil {
		s.logger.Error("refresh token failed",
			"email", claims.Subject,
			"reason", "user_not_found",
		)
		return nil, fmt.Errorf("%w: %w", errors.ErrUserNotFound, err)
	}

	pair, err := s.GenerateTokenPair(user, s.JwtAccessSecret, s.JwtRefreshSecret)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errors.ErrInvalidToken, err)
	}

	s.logger.Info("token refreshed",
		"email", claims.Subject,
	)
	return pair, nil
}

func (s *AuthServiceImpl) SaveState(ctx context.Context, state string) error {
	if err := s.sessionStore.Create(ctx, state); err != nil {
		s.logger.Warn("state store unavailable, continuing without persistence",
			"error", err,
		)
	}
	return nil
}

func (s *AuthServiceImpl) IsValidState(ctx context.Context, callbackState string) (bool, error) {
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

func newUserFromRegistration(req types.RegisterUser) types.User {
	hashedPassword, salt := crypt.HashSHA256WithSalt(req.Password)
	return types.User{
		ID: uuid.NewString(),
		RegisterUser: types.RegisterUser{
			Name:     req.Name,
			Email:    req.Email,
			Password: hashedPassword,
		},
		Salt: salt,
	}
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
