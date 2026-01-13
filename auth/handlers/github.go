package handlers

import (
	cerror "errors"
	"fmt"

	"github.com/Yulian302/lfusys-services-commons/errors"
	jwttypes "github.com/Yulian302/lfusys-services-commons/jwt"
	"github.com/Yulian302/lfusys-services-commons/oauth/github"
	"github.com/Yulian302/lfusys-services-commons/responses"
	"github.com/Yulian302/lfusys-services-gateway/auth/oauth"
	"github.com/Yulian302/lfusys-services-gateway/services"
	"github.com/Yulian302/lfusys-services-gateway/store"
	"github.com/gin-gonic/gin"
)

type GithubHandler struct {
	frontendURL   string
	authSvc       services.AuthService
	userStore     store.UserStore
	oAuthProvider oauth.Provider
}

func NewGithubHandler(frontendUrl string, ghCfg *github.GithubConfig, authSvc services.AuthService, userStore store.UserStore, prov oauth.Provider) *GithubHandler {
	return &GithubHandler{
		frontendURL:   frontendUrl,
		authSvc:       authSvc,
		userStore:     userStore,
		oAuthProvider: prov,
	}
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

	isValid, err := h.authSvc.ValidateState(c, oauth.OAuthPrefix+state)
	if err != nil {
		errors.InternalServerErrorResponse(c, "could not validate state")
		return
	}
	if !isValid {
		errors.UnauthorizedResponse(c, "invalid state")
		return
	}

	token, err := h.oAuthProvider.ExchangeCode(c, code)
	if err != nil {
		errors.UnauthorizedResponse(c, fmt.Sprint("could not retrieve access token: ", err.Error()))
		return
	}

	ghUser, err := h.oAuthProvider.GetOAuthUser(c, token)
	if err != nil {
		errors.InternalServerErrorResponse(c, "could not get user data")
		return
	}

	user, err := h.userStore.GetByEmail(c, ghUser.Email)
	if err != nil {
		if cerror.Is(err, errors.ErrUserNotFound) {
			newUser, err := h.authSvc.RegisterOAuth(c, ghUser)
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
