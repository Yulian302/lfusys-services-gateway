package auth

import (
	"context"

	jwttypes "github.com/Yulian302/lfusys-services-commons/jwt"
	"github.com/Yulian302/lfusys-services-gateway/auth/oauth"
	"github.com/Yulian302/lfusys-services-gateway/auth/types"
)

type LoginResponse struct {
	AccessToken  string
	RefreshToken string
	User         *types.User
}

type JwtAuthService interface {
	Login(ctx context.Context, email string, password string) (*LoginResponse, error)
	Register(ctx context.Context, req types.RegisterUser) error
	GetCurrentUser(ctx context.Context, accessToken string) (*types.User, error)
	RefreshToken(ctx context.Context, refreshToken string) (*jwttypes.TokenPair, error)
}

type StateManager interface {
	SaveState(ctx context.Context, state string) error
	IsValidState(ctx context.Context, callbackState string) (bool, error)
}

type OAuthService interface {
	LoginOAuth(ctx context.Context, email string) (*LoginResponse, error)
	RegisterOAuth(ctx context.Context, userData oauth.OAuthUser) (types.User, error)
}
