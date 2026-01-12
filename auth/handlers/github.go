package handlers

import (
	"context"
	"encoding/json"
	cerror "errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Yulian302/lfusys-services-commons/crypt"
	"github.com/Yulian302/lfusys-services-commons/errors"
	"github.com/Yulian302/lfusys-services-commons/github"
	jwttypes "github.com/Yulian302/lfusys-services-commons/jwt"
	"github.com/Yulian302/lfusys-services-commons/responses"
	"github.com/Yulian302/lfusys-services-gateway/auth/types"
	"github.com/Yulian302/lfusys-services-gateway/services"
	"github.com/Yulian302/lfusys-services-gateway/store"
	"github.com/gin-gonic/gin"
)

type GithubHandler struct {
	ghCfg       *github.GithubConfig
	authService services.AuthService
	userStore   store.UserStore
}

const (
	prefix = "oauth:state:"
)

func NewGithubHandler(ghCfg *github.GithubConfig, authSvc services.AuthService, userStore store.UserStore) *GithubHandler {
	return &GithubHandler{
		ghCfg:       ghCfg,
		authService: authSvc,
		userStore:   userStore,
	}
}

func (h *GithubHandler) NewState(c *gin.Context) {
	state, err := crypt.GenerateState(16)
	if err != nil {
		errors.InternalServerErrorResponse(c, "failed to generate state")
		return
	}

	err = h.authService.SaveState(c, prefix+state)
	if err != nil {
		errors.InternalServerErrorResponse(c, "failed to store state")
		return
	}

	responses.JSONData(c, http.StatusOK, gin.H{
		"state": state,
	})
}

func (h *GithubHandler) Callback(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		errors.UnauthorizedResponse(c, "could not receive `code` from authorizing party")
		return
	}

	state := c.Query("state")
	if state == "" {
		errors.UnauthorizedResponse(c, "could not receive `state` from authorizing party")
		return
	}

	isValid, err := h.authService.ValidateState(c, prefix+state)
	if err != nil {
		errors.InternalServerErrorResponse(c, "could not validate state")
		return
	}
	if !isValid {
		errors.UnauthorizedResponse(c, "invalid state")
		return
	}

	token, err := h.exchangeCodeForToken(c, code)
	if err != nil {
		errors.UnauthorizedResponse(c, fmt.Sprint("could not retrieve access token: ", err.Error()))
		return
	}

	ghUser, err := h.getUser(token)
	if err != nil {
		errors.InternalServerErrorResponse(c, "could not get user data")
		return
	}

	oAuthUser := types.OAuthUser{
		Name:       ghUser.Name,
		Email:      ghUser.Email,
		Provider:   types.Providers[types.GithubProvider],
		ProviderID: strconv.FormatInt(int64(ghUser.ID), 10),
		AvatarURL:  ghUser.AvatarURL,
		Username:   ghUser.Login,
	}

	user, err := h.userStore.GetByEmail(c, ghUser.Email)
	if err != nil {
		if cerror.Is(err, errors.ErrUserNotFound) {
			newUser, err := h.authService.RegisterOAuth(c, oAuthUser)
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

	loginResp, err := h.authService.LoginOAuth(c, user.Email)
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

	responses.Redirect(c, h.ghCfg.FrontendURL)
}

func (h *GithubHandler) exchangeCodeForToken(ctx context.Context, code string) (token string, err error) {
	data := url.Values{}
	data.Set("client_id", h.ghCfg.ClientID)
	data.Set("client_secret", h.ghCfg.ClientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", h.ghCfg.RedirectURI)

	req, err := http.NewRequestWithContext(ctx, "POST", h.ghCfg.ExchangeURL, strings.NewReader(data.Encode()))
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
		return "", fmt.Errorf("github error: %s - %s", tokenResp.Error, tokenResp.ErrorDesc)
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("empty access token received")
	}

	return tokenResp.AccessToken, nil
}

func (h *GithubHandler) getUser(token string) (*types.GithubUser, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
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
		return nil, fmt.Errorf("github API error: %s - %s", resp.Status, string(body))
	}

	var user types.GithubUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to parse user response: %w", err)
	}

	if user.Email == "" {
		user.Email, err = h.getUserEmail(token)
		if err != nil {
			return nil, fmt.Errorf("failed to get user email: %w", err)
		}
	}

	return &user, nil
}

func (h *GithubHandler) getUserEmail(accessToken string) (string, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", err
	}

	for _, email := range emails {
		if email.Primary && email.Verified {
			return email.Email, nil
		}
	}

	for _, email := range emails {
		if email.Verified {
			return email.Email, nil
		}
	}

	return "", fmt.Errorf("no verified email found")
}
