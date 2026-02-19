package handlers

import (
	error "errors"
	"net/http"

	"github.com/Yulian302/lfusys-services-commons/crypt"
	"github.com/Yulian302/lfusys-services-commons/errors"
	jwttypes "github.com/Yulian302/lfusys-services-commons/jwt"
	logger "github.com/Yulian302/lfusys-services-commons/logging"
	"github.com/Yulian302/lfusys-services-commons/responses"
	"github.com/Yulian302/lfusys-services-gateway/auth/oauth"
	"github.com/Yulian302/lfusys-services-gateway/auth/types"
	"github.com/Yulian302/lfusys-services-gateway/services/auth"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	jwtAuthSvc   auth.JwtAuthService
	stateManager auth.StateManager

	logger logger.Logger
}

func NewAuthHandler(jwt auth.JwtAuthService, state auth.StateManager, l logger.Logger) *AuthHandler {
	return &AuthHandler{
		jwtAuthSvc:   jwt,
		stateManager: state,
		logger:       l,
	}
}

func (h *AuthHandler) Me(c *gin.Context) {
	token, err := c.Cookie("jwt")
	if err != nil || token == "" {
		h.logger.Warn("user authorization failed",
			"ip", c.ClientIP(),
			"reason", "bad_request",
		)
		errors.UnauthorizedResponse(c, "unauthorized")
		return
	}

	user, err := h.jwtAuthSvc.GetCurrentUser(c.Request.Context(), token)
	if err != nil {
		if error.Is(err, errors.ErrUserNotFound) || error.Is(err, errors.ErrInvalidToken) {
			h.logger.Warn("user authorization failed",
				"email", user.Email,
				"reason", "unauthorized",
			)
			errors.UnauthorizedResponse(c, err.Error())
		} else {
			h.logger.Error("user authorization failed",
				"email", user.Email,
				"reason", "internal_server",
			)
			errors.InternalServerErrorResponse(c, err.Error())
		}
		return
	}

	h.logger.Info("user authorized",
		"email", user.Email,
	)

	responses.JSONData(c, http.StatusOK, types.MeResponse{
		Email:         user.Email,
		Name:          user.Name,
		Authenticated: true,
	})

}

func (h *AuthHandler) Register(c *gin.Context) {
	var req types.RegisterUser
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("user registration failed",
			"email", req.Email,
			"reason", "bad_request",
		)
		errors.BadRequestResponse(c, err.Error())
		return
	}

	if err := h.jwtAuthSvc.Register(c.Request.Context(), req); err != nil {
		if error.Is(err, errors.ErrUserAlreadyExists) {
			h.logger.Warn("user registration failed",
				"email", req.Email,
				"reason", "user_already_exists",
			)
			errors.ConflictResponse(c, err.Error())
		} else {
			h.logger.Error("user registration failed",
				"email", req.Email,
				"reason", "internal_server",
			)
			errors.InternalServerErrorResponse(c, err.Error())
		}
		return
	}
	h.logger.Info("user registered",
		"email", req.Email,
	)

	responses.JSONCreated(c, "created")
}

func (h *AuthHandler) Login(c *gin.Context) {
	var loginUser types.LoginUser

	if err := c.ShouldBindJSON(&loginUser); err != nil {
		h.logger.Warn("user login failed",
			"email", loginUser.Email,
			"reason", "bad_request",
		)
		errors.BadRequestResponse(c, err.Error())
		return
	}

	loginResp, err := h.jwtAuthSvc.Login(c.Request.Context(), loginUser.Email, loginUser.Password)
	if err != nil {
		if error.Is(err, errors.ErrInvalidCredentials) {
			h.logger.Warn("user login failed",
				"email", loginUser.Email,
				"reason", "invalid_credentials",
			)
			errors.UnauthorizedResponse(c, err.Error())
		} else {
			h.logger.Error("user login failed",
				"email", loginUser.Email,
				"reason", "internal_server",
			)
			errors.InternalServerErrorResponse(c, err.Error())
		}
		return
	}

	// set refresh token (30 days)
	c.SetCookie(
		"refresh_token",
		loginResp.RefreshToken,
		int(jwttypes.RefreshTokenDuration),
		jwttypes.CookiePath,
		"",
		false,
		true,
	)

	// set access token (30 mins)
	c.SetCookie(
		"jwt",
		loginResp.AccessToken,
		int(jwttypes.AccessTokenDuration),
		jwttypes.CookiePath,
		"",
		false,
		true,
	)
	h.logger.Info("user logged in",
		"email", loginUser.Email,
	)
	responses.JSONSuccess(c, "login successful")
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	oldRefreshToken, err := c.Cookie("refresh_token")
	if err != nil || oldRefreshToken == "" {
		h.logger.Warn("user refresh token failed",
			"ip", c.ClientIP(),
			"reason", "missing token",
		)
		errors.UnauthorizedResponse(c, "missing refresh token")
		return
	}

	tokenPair, err := h.jwtAuthSvc.RefreshToken(c.Request.Context(), oldRefreshToken)
	if err != nil {
		if error.Is(err, errors.ErrUserNotFound) || error.Is(err, errors.ErrInvalidToken) || error.Is(err, errors.ErrInvalidTokenType) {
			h.logger.Warn("user refresh token failed",
				"ip", c.ClientIP(),
				"reason", "invalid token",
			)
			errors.ConflictResponse(c, err.Error())
		} else {
			h.logger.Error("user refresh token failed",
				"ip", c.ClientIP(),
				"reason", "internal_server",
			)
			errors.InternalServerErrorResponse(c, err.Error())
		}
		return
	}

	c.SetCookie("jwt", tokenPair.AccessToken, int(jwttypes.AccessTokenDuration), jwttypes.CookiePath, "", false, true)
	c.SetCookie("refresh_token", tokenPair.RefreshToken, int(jwttypes.RefreshTokenDuration), jwttypes.CookiePath, "", false, true)

	h.logger.Info("user refreshed token",
		"ip", c.ClientIP(),
	)
	responses.JSONSuccess(c, "token refreshed")
}

func (h *AuthHandler) Logout(c *gin.Context) {
	c.SetCookie("jwt", "", -1, jwttypes.CookiePath, "", false, true)
	c.SetCookie("refresh_token", "", -1, jwttypes.CookiePath, "", false, true)

	h.logger.Info("user logged out",
		"ip", c.ClientIP(),
	)

	responses.JSONSuccess(c, "logged out")
}

func (h *AuthHandler) NewState(c *gin.Context) {
	state, err := crypt.GenerateState(16)
	if err != nil {
		h.logger.Error("state generation failed",
			"ip", c.ClientIP(),
			"reason", "internal_server",
		)
		errors.InternalServerErrorResponse(c, "failed to generate state")
		return
	}

	err = h.stateManager.SaveState(c.Request.Context(), oauth.OAuthPrefix+state)
	if err != nil {
		h.logger.Error("state generation failed",
			"ip", c.ClientIP(),
			"reason", "internal_server",
		)
		errors.InternalServerErrorResponse(c, "failed to store state")
		return
	}

	h.logger.Info("state generated",
		"ip", c.ClientIP(),
	)
	responses.JSONData(c, http.StatusOK, gin.H{
		"state": state,
	})
}
