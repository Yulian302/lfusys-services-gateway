package oauth

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/Yulian302/lfusys-services-commons/config"
	"github.com/Yulian302/lfusys-services-gateway/auth"
	"github.com/Yulian302/lfusys-services-gateway/auth/types"
)

type githubProvider struct {
	cfg    *config.GithubConfig
	client *auth.Client
}

func NewGithubProvider(cfg *config.GithubConfig) *githubProvider {
	return &githubProvider{
		cfg:    cfg,
		client: auth.NewClient(10 * time.Second),
	}
}

func (p *githubProvider) ExchangeCode(ctx context.Context, code string) (string, error) {
	data := url.Values{}
	data.Set("client_id", p.cfg.ClientID)
	data.Set("client_secret", p.cfg.ClientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", p.cfg.RedirectURI)

	var resp struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}

	if err := p.client.PostFormJSON(ctx, p.cfg.ExchangeURL, data, &resp); err != nil {
		return "", err
	}
	if resp.Error != "" {
		return "", fmt.Errorf("github error: %s - %s", resp.Error, resp.ErrorDesc)
	}
	if resp.AccessToken == "" {
		return "", fmt.Errorf("empty access token")
	}
	return resp.AccessToken, nil
}

func (p *githubProvider) GetOAuthUser(ctx context.Context, token string) (OAuthUser, error) {
	var ghUser types.GithubUser
	if err := p.client.GetJSONWithToken(ctx, "https://api.github.com/user", token, &ghUser); err != nil {
		return OAuthUser{}, err
	}

	user := OAuthUser{
		Name:       ghUser.Name,
		Email:      ghUser.Email,
		Provider:   types.Providers[types.GithubProvider],
		ProviderID: strconv.FormatInt(int64(ghUser.ID), 10),
		AvatarURL:  ghUser.AvatarURL,
		Username:   ghUser.Login,
	}

	if user.Email == "" {
		var emails []struct {
			Email    string `json:"email"`
			Primary  bool   `json:"primary"`
			Verified bool   `json:"verified"`
		}
		if err := p.client.GetJSONWithToken(ctx, "https://api.github.com/user/emails", token, &emails); err != nil {
			return OAuthUser{}, err
		}
		for _, e := range emails {
			if e.Primary && e.Verified {
				user.Email = e.Email
				break
			}
		}
		if user.Email == "" {
			for _, e := range emails {
				if e.Verified {
					user.Email = e.Email
					break
				}
			}
		}
		if user.Email == "" {
			return OAuthUser{}, fmt.Errorf("no verified email found")
		}
	}

	return user, nil

}
