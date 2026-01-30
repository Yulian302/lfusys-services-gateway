package handlers

import (
	cerror "errors"
	"fmt"

	"github.com/Yulian302/lfusys-services-commons/config"
	"github.com/Yulian302/lfusys-services-commons/errors"
	jwttypes "github.com/Yulian302/lfusys-services-commons/jwt"
	"github.com/Yulian302/lfusys-services-commons/responses"
	"github.com/Yulian302/lfusys-services-gateway/auth/oauth"
	"github.com/Yulian302/lfusys-services-gateway/auth/types"
	"github.com/Yulian302/lfusys-services-gateway/services"
	"github.com/Yulian302/lfusys-services-gateway/store"
	"github.com/gin-gonic/gin"
)

type GoogleHandler struct {
	frontendURL   string
	authSvc       services.AuthService
	userStore     store.UserStore
	oauthProvider oauth.Provider
}

func NewGoogleHandler(frontendURL string, ghCfg *config.GoogleConfig, authSvc services.AuthService, userStore store.UserStore, prov oauth.Provider) *GoogleHandler {
	return &GoogleHandler{
		frontendURL:   frontendURL,
		authSvc:       authSvc,
		userStore:     userStore,
		oauthProvider: prov,
	}
}

func (h *GoogleHandler) Callback(c *gin.Context) {
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

	isValid, err := h.authSvc.IsValidState(c, oauth.OAuthPrefix+state)
	if err != nil {
		errors.InternalServerErrorResponse(c, "could not validate state")
		return
	}
	if !isValid {
		errors.UnauthorizedResponse(c, "invalid state")
		return
	}

	token, err := h.oauthProvider.ExchangeCode(c, code)
	if err != nil {
		errors.UnauthorizedResponse(c, fmt.Sprint("could not retrieve access token: ", err.Error()))
		return
	}

	gUser, err := h.oauthProvider.GetOAuthUser(c, token)
	if err != nil {
		errors.InternalServerErrorResponse(c, "could not get user data")
		return
	}

	oAuthUser := oauth.OAuthUser{
		Name:       gUser.Name,
		Email:      gUser.Email,
		Provider:   types.Providers[types.GithubProvider],
		ProviderID: gUser.ProviderID,
		AvatarURL:  gUser.AvatarURL,
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

	responses.Redirect(c, h.frontendURL)
}
