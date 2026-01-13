package oauth

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/Yulian302/lfusys-services-commons/oauth/google"
	"github.com/Yulian302/lfusys-services-gateway/auth"
	"github.com/Yulian302/lfusys-services-gateway/auth/types"
)

type googleProvider struct {
	cfg    *google.GoogleConfig
	client *auth.Client
}

func NewGoogleProvider(cfg *google.GoogleConfig) *googleProvider {
	return &googleProvider{
		cfg:    cfg,
		client: auth.NewClient(10 * time.Second),
	}
}

func (p *googleProvider) ExchangeCode(ctx context.Context, code string) (string, error) {
	data := url.Values{}
	data.Set("client_id", p.cfg.ClientID)
	data.Set("client_secret", p.cfg.ClientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", p.cfg.RedirectURI)
	data.Set("grant_type", "authorization_code")

	var resp struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}

	if err := p.client.PostFormJSON(ctx, p.cfg.ExchangeURL, data, &resp); err != nil {
		return "", err
	}
	if resp.Error != "" {
		return "", fmt.Errorf("google error: %s - %s", resp.Error, resp.ErrorDesc)
	}
	if resp.AccessToken == "" {
		return "", fmt.Errorf("empty access token")
	}
	return resp.AccessToken, nil
}

func (p *googleProvider) GetOAuthUser(ctx context.Context, token string) (OAuthUser, error) {
	var gUser types.GoogleUser
	if err := p.client.GetJSONWithToken(ctx, "https://www.googleapis.com/oauth2/v3/userinfo", token, &gUser); err != nil {
		return OAuthUser{}, err
	}

	if gUser.ID == "" {
		return OAuthUser{}, fmt.Errorf("Google response missing user ID")
	}

	if gUser.Email == "" {
		return OAuthUser{}, fmt.Errorf("Google response missing email")
	}

	if !gUser.EmailVerified {
		return OAuthUser{}, fmt.Errorf("email %s not verified by Google", gUser.Email)
	}

	user := OAuthUser{
		Name:          gUser.Name,
		Email:         gUser.Email,
		Provider:      types.Providers[types.GoogleProvider],
		ProviderID:    gUser.ID,
		AvatarURL:     gUser.Picture,
		Username:      gUser.Name,
		EmailVerified: true,
	}

	return user, nil

}
