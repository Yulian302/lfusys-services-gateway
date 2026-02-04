package handlers

import (
	error "errors"
	"net/http"

	"github.com/Yulian302/lfusys-services-commons/crypt"
	"github.com/Yulian302/lfusys-services-commons/errors"
	jwttypes "github.com/Yulian302/lfusys-services-commons/jwt"
	"github.com/Yulian302/lfusys-services-commons/responses"
	"github.com/Yulian302/lfusys-services-gateway/auth/oauth"
	"github.com/Yulian302/lfusys-services-gateway/auth/types"
	"github.com/Yulian302/lfusys-services-gateway/services"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService services.AuthService
}

func NewAuthHandler(authService services.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

func (h *AuthHandler) Me(c *gin.Context) {
	token, err := c.Cookie("jwt")
	if err != nil || token == "" {
		errors.UnauthorizedResponse(c, "unauthorized")
		return
	}

	user, err := h.authService.GetCurrentUser(c.Request.Context(), token)
	if err != nil {
		if error.Is(err, errors.ErrUserNotFound) || error.Is(err, errors.ErrInvalidToken) {
			errors.UnauthorizedResponse(c, err.Error())
		} else {
			errors.InternalServerErrorResponse(c, err.Error())
		}
		return
	}

	responses.JSONData(c, http.StatusOK, types.MeResponse{
		Email:         user.Email,
		Name:          user.Name,
		Authenticated: true,
	})

}

func (h *AuthHandler) Register(c *gin.Context) {
	var req types.RegisterUser
	if err := c.ShouldBindJSON(&req); err != nil {
		errors.BadRequestResponse(c, err.Error())
		return
	}

	if err := h.authService.Register(c.Request.Context(), req); err != nil {
		if error.Is(err, errors.ErrUserAlreadyExists) {
			errors.ConflictResponse(c, err.Error())
		} else {
			errors.InternalServerErrorResponse(c, err.Error())
		}
		return
	}

	responses.JSONCreated(c, "created")
}

func (h *AuthHandler) Login(c *gin.Context) {
	var loginUser types.LoginUser

	if err := c.ShouldBindJSON(&loginUser); err != nil {
		errors.BadRequestResponse(c, err.Error())
		return
	}

	loginResp, err := h.authService.Login(c.Request.Context(), loginUser.Email, loginUser.Password)
	if err != nil {
		if error.Is(err, errors.ErrInvalidCredentials) {
			errors.UnauthorizedResponse(c, err.Error())
		} else {
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

	responses.JSONSuccess(c, "login successful")
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	oldRefreshToken, err := c.Cookie("refresh_token")
	if err != nil || oldRefreshToken == "" {
		errors.UnauthorizedResponse(c, "missing refresh token")
		return
	}

	tokenPair, err := h.authService.RefreshToken(c.Request.Context(), oldRefreshToken)
	if err != nil {
		if error.Is(err, errors.ErrUserNotFound) || error.Is(err, errors.ErrInvalidToken) || error.Is(err, errors.ErrInvalidTokenType) {
			errors.ConflictResponse(c, err.Error())
		} else {
			errors.InternalServerErrorResponse(c, err.Error())
		}
		return
	}

	c.SetCookie("jwt", tokenPair.AccessToken, int(jwttypes.AccessTokenDuration), jwttypes.CookiePath, "", false, true)
	c.SetCookie("refresh_token", tokenPair.RefreshToken, int(jwttypes.RefreshTokenDuration), jwttypes.CookiePath, "", false, true)
	responses.JSONSuccess(c, "token refreshed")
}

func (h *AuthHandler) Logout(ctx *gin.Context) {
	ctx.SetCookie("jwt", "", -1, jwttypes.CookiePath, "", false, true)
	ctx.SetCookie("refresh_token", "", -1, jwttypes.CookiePath, "", false, true)
	responses.JSONSuccess(ctx, "logged out")
}

func (h *AuthHandler) NewState(c *gin.Context) {
	state, err := crypt.GenerateState(16)
	if err != nil {
		errors.InternalServerErrorResponse(c, "failed to generate state")
		return
	}

	err = h.authService.SaveState(c.Request.Context(), oauth.OAuthPrefix+state)
	if err != nil {
		errors.InternalServerErrorResponse(c, "failed to store state")
		return
	}

	responses.JSONData(c, http.StatusOK, gin.H{
		"state": state,
	})
}
