package oauth

import "context"

type OAuthUser struct {
	Name          string
	Email         string
	EmailVerified bool
	Provider      string
	ProviderID    string
	AvatarURL     string
	Username      string
}

const (
	OAuthPrefix = "oauth:state:"
)

type Provider interface {
	ExchangeCode(ctx context.Context, code string) (accessToken string, err error)
	GetOAuthUser(ctx context.Context, token string) (OAuthUser, error)
}
