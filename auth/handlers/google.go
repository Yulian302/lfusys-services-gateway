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
	"github.com/Yulian302/lfusys-services-gateway/auth/types"
	"github.com/Yulian302/lfusys-services-gateway/services/auth"
	"github.com/Yulian302/lfusys-services-gateway/store"
	"github.com/gin-gonic/gin"
)

type GoogleHandler struct {
	frontendURL   string
	oAuthSvc      auth.OAuthService
	stateManager  auth.StateManager
	userStore     store.UserStore
	oauthProvider oauth.Provider

	logger logger.Logger
}

func NewGoogleHandler(frontendURL string, ghCfg config.GoogleConfig, oauth auth.OAuthService, state auth.StateManager, userStore store.UserStore, prov oauth.Provider, l logger.Logger) *GoogleHandler {
	return &GoogleHandler{
		frontendURL:   frontendURL,
		oAuthSvc:      oauth,
		stateManager:  state,
		userStore:     userStore,
		oauthProvider: prov,
		logger:        l,
	}
}

func (h *GoogleHandler) Callback(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		h.logger.Warn("google oauth callback failed",
			"ip", c.ClientIP(),
			"reason", "bad_request",
		)
		errors.UnauthorizedResponse(c, "could not receive `code` from authorizing party")
		return
	}

	state := c.Query("state")
	if state == "" {
		h.logger.Warn("google oauth callback failed",
			"ip", c.ClientIP(),
			"reason", "bad_request",
		)
		errors.UnauthorizedResponse(c, "could not receive `state` from authorizing party")
		return
	}

	ctx := c.Request.Context()

	isValid, err := h.stateManager.IsValidState(ctx, oauth.OAuthPrefix+state)
	if err != nil {
		h.logger.Error("google oauth callback failed",
			"ip", c.ClientIP(),
			"reason", "internal_server",
		)
		errors.InternalServerErrorResponse(c, "could not validate state")
		return
	}
	if !isValid {
		h.logger.Warn("google oauth callback failed",
			"ip", c.ClientIP(),
			"reason", "invalid state",
		)
		errors.UnauthorizedResponse(c, "invalid state")
		return
	}

	token, err := h.oauthProvider.ExchangeCode(ctx, code)
	if err != nil {
		h.logger.Warn("google oauth callback failed",
			"ip", c.ClientIP(),
			"reason", "wrong access token",
		)
		errors.UnauthorizedResponse(c, fmt.Sprint("could not retrieve access token: ", err.Error()))
		return
	}

	gUser, err := h.oauthProvider.GetOAuthUser(ctx, token)
	if err != nil {
		h.logger.Error("google oauth callback failed",
			"ip", c.ClientIP(),
			"reason", "internal_server",
		)
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

	user, err := h.userStore.GetByEmail(ctx, gUser.Email)
	if err != nil {
		if cerror.Is(err, errors.ErrUserNotFound) {
			h.logger.Info("oauth user not found, creating one...",
				"ip", c.ClientIP(),
			)
			newUser, err := h.oAuthSvc.RegisterOAuth(ctx, oAuthUser)
			if err != nil {
				h.logger.Error("google oauth callback failed",
					"ip", c.ClientIP(),
					"reason", "user creation failed",
				)
				errors.InternalServerErrorResponse(c, "failed to create user")
				return
			}
			user = &newUser
		} else {
			h.logger.Error("google oauth callback failed",
				"ip", c.ClientIP(),
				"reason", "database failure",
			)
			errors.InternalServerErrorResponse(c, "database failure")
			return
		}
	}

	loginResp, err := h.oAuthSvc.LoginOAuth(ctx, user.Email)
	if err != nil {
		h.logger.Error("google oauth callback failed",
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
	h.logger.Info("google oauth callback success",
		"ip", c.ClientIP(),
	)

	responses.Redirect(c, h.frontendURL)
}
