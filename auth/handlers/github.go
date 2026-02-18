package handlers

import (
	cerror "errors"
	"fmt"

	"github.com/Yulian302/lfusys-services-commons/config"
	"github.com/Yulian302/lfusys-services-commons/errors"
	jwttypes "github.com/Yulian302/lfusys-services-commons/jwt"
	logger "github.com/Yulian302/lfusys-services-commons/logging"
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

	logger logger.Logger
}

func NewGithubHandler(frontendUrl string, ghCfg *config.GithubConfig, authSvc services.AuthService, userStore store.UserStore, prov oauth.Provider, l logger.Logger) *GithubHandler {
	return &GithubHandler{
		frontendURL:   frontendUrl,
		authSvc:       authSvc,
		userStore:     userStore,
		oAuthProvider: prov,
		logger:        l,
	}
}

func (h *GithubHandler) Callback(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		h.logger.Warn("github oauth callback failed",
			"ip", c.ClientIP(),
			"reason", "bad_request",
		)
		errors.UnauthorizedResponse(c, "could not receive `code` from authorizing party")
		return
	}

	state := c.Query("state")
	if state == "" {
		h.logger.Warn("github oauth callback failed",
			"ip", c.ClientIP(),
			"reason", "bad_request",
		)
		errors.UnauthorizedResponse(c, "could not receive `state` from authorizing party")
		return
	}

	ctx := c.Request.Context()

	isValid, err := h.authSvc.IsValidState(ctx, oauth.OAuthPrefix+state)
	if err != nil {
		h.logger.Error("github oauth callback failed",
			"ip", c.ClientIP(),
			"reason", "internal_server",
		)
		errors.InternalServerErrorResponse(c, "could not validate state")
		return
	}
	if !isValid {
		h.logger.Warn("github oauth callback failed",
			"ip", c.ClientIP(),
			"reason", "invalid state",
		)
		errors.UnauthorizedResponse(c, "invalid state")
		return
	}

	token, err := h.oAuthProvider.ExchangeCode(ctx, code)
	if err != nil {
		h.logger.Warn("github oauth callback failed",
			"ip", c.ClientIP(),
			"reason", "wrong access token",
		)
		errors.UnauthorizedResponse(c, fmt.Sprint("could not retrieve access token: ", err.Error()))
		return
	}

	ghUser, err := h.oAuthProvider.GetOAuthUser(ctx, token)
	if err != nil {
		h.logger.Error("github oauth callback failed",
			"ip", c.ClientIP(),
			"reason", "internal_server",
		)
		errors.InternalServerErrorResponse(c, "could not get user data")
		return
	}

	user, err := h.userStore.GetByEmail(ctx, ghUser.Email)
	if err != nil {
		if cerror.Is(err, errors.ErrUserNotFound) {
			h.logger.Info("oauth user not found, creating one...",
				"ip", c.ClientIP(),
			)
			newUser, err := h.authSvc.RegisterOAuth(ctx, ghUser)
			if err != nil {
				h.logger.Error("github oauth callback failed",
					"ip", c.ClientIP(),
					"reason", "user creation failed",
				)
				errors.InternalServerErrorResponse(c, "failed to create user")
				return
			}
			user = &newUser
		} else {
			h.logger.Error("github oauth callback failed",
				"ip", c.ClientIP(),
				"reason", "database failure",
			)
			errors.InternalServerErrorResponse(c, "database failure")
			return
		}
	}

	loginResp, err := h.authSvc.LoginOAuth(ctx, user.Email)
	if err != nil {
		h.logger.Error("github oauth callback failed",
			"ip", c.ClientIP(),
			"reason", "internal_server",
		)
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
	h.logger.Info("github oauth callback success",
		"ip", c.ClientIP(),
	)

	responses.Redirect(c, h.frontendURL)
}
