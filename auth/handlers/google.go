package handlers

import (
	"context"
	"encoding/json"
	cerror "errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Yulian302/lfusys-services-commons/errors"
	"github.com/Yulian302/lfusys-services-commons/google"
	jwttypes "github.com/Yulian302/lfusys-services-commons/jwt"
	"github.com/Yulian302/lfusys-services-commons/responses"
	"github.com/Yulian302/lfusys-services-gateway/auth/types"
	"github.com/Yulian302/lfusys-services-gateway/services"
	"github.com/Yulian302/lfusys-services-gateway/store"
	"github.com/gin-gonic/gin"
)

type GoogleHandler struct {
	cfg       *google.GoogleConfig
	authSvc   services.AuthService
	userStore store.UserStore
}

const (
	googlePrefix = "oauth:google:"
)

func NewGoogleHandler(ghCfg *google.GoogleConfig, authSvc services.AuthService, userStore store.UserStore) *GoogleHandler {
	return &GoogleHandler{
		cfg:       ghCfg,
		authSvc:   authSvc,
		userStore: userStore,
	}
}

func (h *GoogleHandler) Callback(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		errors.UnauthorizedResponse(c, "could not receive `code` from authorizing party")
		return
	}

	// state := c.Query("state")
	// if state == "" {
	// 	errors.UnauthorizedResponse(c, "could not receive `state` from authorizing party")
	// 	return
	// }

	// isValid, err := h.authService.ValidateState(c, githubPrefix+state)
	// if err != nil {
	// 	errors.InternalServerErrorResponse(c, "could not validate state")
	// 	return
	// }
	// if !isValid {
	// 	errors.UnauthorizedResponse(c, "invalid state")
	// 	return
	// }

	token, err := h.exchangeCodeForTokenGoogle(c, code)
	if err != nil {
		errors.UnauthorizedResponse(c, fmt.Sprint("could not retrieve access token: ", err.Error()))
		return
	}

	gUser, err := h.getUserGoogle(token)
	if err != nil {
		errors.InternalServerErrorResponse(c, "could not get user data")
		return
	}

	oAuthUser := types.OAuthUser{
		Name:       gUser.Name,
		Email:      gUser.Email,
		Provider:   types.Providers[types.GithubProvider],
		ProviderID: gUser.ID,
		AvatarURL:  gUser.Picture,
		Username:   gUser.Name,
	}

	user, err := h.userStore.GetByEmail(c, gUser.Email)
	if err != nil {
		if cerror.Is(err, errors.ErrUserNotFound) {
			newUser, err := h.authSvc.RegisterOAuth(c, oAuthUser)
			if err != nil {
				errors.InternalServerErrorResponse(c, "failed to create user")
				return
			}
			user = &newUser
		} else {
			errors.InternalServerErrorResponse(c, "database failure")
			return
		}
	}

	loginResp, err := h.authSvc.LoginOAuth(c, user.Email)
	if err != nil {
		errors.InternalServerErrorResponse(c, "failed to generate session")
		return
	}

	c.SetCookie(
		"refresh_token",
		loginResp.RefreshToken,
		int(jwttypes.RefreshTokenDuration),
		jwttypes.CookiePath,
		"",
		false,
		true,
	)

	c.SetCookie(
		"jwt",
		loginResp.AccessToken,
		int(jwttypes.AccessTokenDuration),
		jwttypes.CookiePath,
		"",
		false,
		true,
	)

	responses.Redirect(c, h.cfg.FrontendURL)
}

func (h *GoogleHandler) exchangeCodeForTokenGoogle(ctx context.Context, code string) (token string, err error) {
	data := url.Values{}
	data.Set("client_id", h.cfg.ClientID)
	data.Set("client_secret", h.cfg.ClientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", h.cfg.RedirectURI)
	data.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(ctx, "POST", h.cfg.ExchangeURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("token exchange request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read token response: %w", err)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Scope       string `json:"scope"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}

	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}

	if tokenResp.Error != "" {
		return "", fmt.Errorf("google error: %s - %s", tokenResp.Error, tokenResp.ErrorDesc)
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("empty access token received")
	}

	return tokenResp.AccessToken, nil
}

func (h *GoogleHandler) getUserGoogle(token string) (*types.GoogleUser, error) {
	req, err := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v3/userinfo", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create user request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("user request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Println(fmt.Printf("google API error: %s - %s", resp.Status, string(body)))
		return nil, fmt.Errorf("google API error: %s - %s", resp.Status, string(body))
	}

	var user types.GoogleUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to parse user response: %w", err)
	}

	if !user.EmailVerified && user.Email != "" {
		return nil, fmt.Errorf("email not verified by Google")
	}

	return &user, nil
}
