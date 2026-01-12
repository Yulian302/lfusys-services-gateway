package services

import (
	"context"
	cerr "errors"
	"fmt"
	"log"
	"time"

	"github.com/Yulian302/lfusys-services-commons/crypt"
	"github.com/Yulian302/lfusys-services-commons/errors"
	jwttypes "github.com/Yulian302/lfusys-services-commons/jwt"
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

type AuthService interface {
	Login(ctx context.Context, email string, password string) (*LoginResponse, error)
	LoginOAuth(ctx context.Context, email string) (*LoginResponse, error)
	Register(ctx context.Context, req types.RegisterUser) error
	RegisterOAuth(ctx context.Context, userData types.OAuthUser) (types.User, error)
	GetCurrentUser(ctx context.Context, accessToken string) (*types.User, error)
	RefreshToken(ctx context.Context, refreshToken string) (*jwttypes.TokenPair, error)
	SaveState(ctx context.Context, state string) error
	ValidateState(ctx context.Context, callbackState string) (bool, error)
}

type AuthServiceImpl struct {
	userStore        store.UserStore
	sessionStore     store.SessionStore
	JwtAccessSecret  string
	JwtRefreshSecret string
}

func NewAuthServiceImpl(userStore store.UserStore, sessionStore store.SessionStore, jwtAccessSecret, jwtRefreshSecret string) *AuthServiceImpl {
	return &AuthServiceImpl{
		userStore:        userStore,
		sessionStore:     sessionStore,
		JwtAccessSecret:  jwtAccessSecret,
		JwtRefreshSecret: jwtRefreshSecret,
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
		log.Printf("could not sign JWT token: %v", err)
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
		log.Printf("could not sign refresh token: %v", err)
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
		return nil, fmt.Errorf("%w: %w", errors.ErrInvalidCredentials, err)
	}

	if !crypt.VerifyPasswordWithSalt(password, user.Password, user.Salt) {
		return nil, fmt.Errorf("%w: %w", errors.ErrInvalidCredentials, err)
	}

	tokenPair, err := s.GenerateTokenPair(user, s.JwtAccessSecret, s.JwtRefreshSecret)
	if err != nil {
		return nil, err
	}

	return &LoginResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		User:         user,
	}, nil
}

func (s *AuthServiceImpl) LoginOAuth(ctx context.Context, email string) (*LoginResponse, error) {
	user, err := s.userStore.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	tokenPair, err := s.GenerateTokenPair(user, s.JwtAccessSecret, s.JwtRefreshSecret)
	if err != nil {
		return nil, fmt.Errorf("generating token pair: %w", err)
	}

	return &LoginResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		User:         user,
	}, nil
}

func (s *AuthServiceImpl) Register(ctx context.Context, req types.RegisterUser) error {
	var user types.User
	user.Email = req.Email
	user.Name = req.Name
	user.Password, user.Salt = crypt.HashSHA256WithSalt(req.Password)
	uuidVal := uuid.New()
	user.ID = uuidVal.String()

	err := s.userStore.Create(ctx, user)
	if err != nil {
		if cerr.Is(err, errors.ErrUserAlreadyExists) {
			return fmt.Errorf("%w: %w", errors.ErrUserAlreadyExists, err)
		} else {
			return fmt.Errorf("%w: %w", errors.ErrInternalServer, err)
		}
	}

	return nil
}

func (s *AuthServiceImpl) RegisterOAuth(ctx context.Context, userData types.OAuthUser) (types.User, error) {
	user := types.User{
		ID: uuid.NewString(),
		RegisterUser: types.RegisterUser{
			Email: userData.Email,
			Name:  userData.Name,
		},
		OAuthProvider: userData.Provider,
		OAuthID:       userData.ProviderID,
		Verified:      true,
	}

	err := s.userStore.Create(ctx, user)
	if err != nil {
		if cerr.Is(err, errors.ErrUserAlreadyExists) {
			return types.User{}, errors.ErrUserAlreadyExists
		}
		return types.User{}, fmt.Errorf("db create user: %w", err)
	}

	return user, nil
}

func (s *AuthServiceImpl) GetCurrentUser(ctx context.Context, accessToken string) (*types.User, error) {

	claims, err := s.ValidateToken(accessToken)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errors.ErrInvalidToken, err)
	}

	user, err := s.userStore.GetByEmail(ctx, claims.Subject)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errors.ErrUserNotFound, err)
	}

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
		return nil, fmt.Errorf("%w: %w", errors.ErrInvalidToken, err)
	}

	if claims.Type != "refresh" {
		return nil, fmt.Errorf("%w: %w", errors.ErrInvalidTokenType, err)
	}

	user, err := s.userStore.GetByEmail(ctx, claims.Subject)
	if err != nil || user == nil {
		return nil, fmt.Errorf("%w: %w", errors.ErrUserNotFound, err)
	}

	pair, err := s.GenerateTokenPair(user, s.JwtAccessSecret, s.JwtRefreshSecret)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errors.ErrInvalidToken, err)
	}
	return pair, nil
}

func (s *AuthServiceImpl) SaveState(ctx context.Context, state string) error {
	err := s.sessionStore.Create(ctx, "oauth:state:", state, "1")
	return err
}

func (s *AuthServiceImpl) ValidateState(ctx context.Context, callbackState string) (bool, error) {
	valid, err := s.sessionStore.Validate(ctx, "oauth:state:", callbackState)
	if err != nil {
		return false, err
	}
	if !valid {
		return false, nil
	}
	return true, nil
}
