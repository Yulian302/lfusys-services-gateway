package services

import (
	"context"
	"fmt"
	"log"
	"strings"
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
}

type AuthServiceImpl struct {
	userStore        store.UserStore
	JwtAccessSecret  string
	JwtRefreshSecret string
}

func NewAuthServiceImpl(userStore store.UserStore, jwtAccessSecret, jwtRefreshSecret string) *AuthServiceImpl {
	return &AuthServiceImpl{
		userStore:        userStore,
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
		return nil, fmt.Errorf("%w: %w", errors.ErrUserNotFound, err)
	}

	if !crypt.VerifyPasswordWithSalt(password, user.Password, user.Salt) {
		return nil, fmt.Errorf("%w: %w", errors.ErrUserNotFound, err)
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

func (s *AuthServiceImpl) Register(ctx context.Context, req types.RegisterUser) error {
	var user types.User
	user.Email = req.Email
	user.Name = req.Name
	user.Password, user.Salt = crypt.HashPasswordWithSalt(req.Password)
	uuidVal := uuid.New()
	user.ID = uuidVal.String()

	err := s.userStore.Create(ctx, user)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("%w: %w", errors.ErrUserAlreadyExists, err)
		}
		return fmt.Errorf("%w: %w", errors.ErrInternalServer, err)
	}

	return nil
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
