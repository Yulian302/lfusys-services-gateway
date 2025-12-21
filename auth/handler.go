package auth

import (
	error "errors"
	"net/http"

	"github.com/Yulian302/lfusys-services-commons/errors"
	jwttypes "github.com/Yulian302/lfusys-services-commons/jwt"
	"github.com/Yulian302/lfusys-services-commons/responses"
	"github.com/Yulian302/lfusys-services-gateway/auth/types"
	"github.com/Yulian302/lfusys-services-gateway/services"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService services.AuthService
}

func NewAuthHandler(authService *services.AuthServiceImpl) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

func (h *AuthHandler) Me(ctx *gin.Context) {
	token, err := ctx.Cookie("jwt")
	if err != nil || token == "" {
		errors.UnauthorizedResponse(ctx, "unauthorized")
		return
	}

	user, err := h.authService.GetCurrentUser(ctx, token)
	if err != nil {
		if error.Is(err, errors.ErrUserNotFound) || error.Is(err, errors.ErrInvalidToken) {
			errors.UnauthorizedResponse(ctx, err.Error())
		} else {
			errors.InternalServerErrorResponse(ctx, err.Error())
		}
		return
	}

	responses.JSONData(ctx, http.StatusOK, types.MeResponse{
		Email:         user.Email,
		Name:          user.Name,
		Authenticated: true,
	})

}

func (h *AuthHandler) Register(ctx *gin.Context) {
	var req types.RegisterUser
	if err := ctx.ShouldBindJSON(&req); err != nil {
		errors.BadRequestResponse(ctx, err.Error())
		return
	}

	if err := h.authService.Register(ctx, req); err != nil {
		if error.Is(err, errors.ErrUserAlreadyExists) {
			errors.ConflictResponse(ctx, err.Error())
		} else {
			errors.InternalServerErrorResponse(ctx, err.Error())
		}
		return
	}

	responses.JSONCreated(ctx, "created")
}

func (h *AuthHandler) Login(ctx *gin.Context) {
	var loginUser types.LoginUser

	if err := ctx.ShouldBindJSON(&loginUser); err != nil {
		errors.BadRequestResponse(ctx, err.Error())
		return
	}

	loginResp, err := h.authService.Login(ctx, loginUser.Email, loginUser.Password)
	if err != nil {
		if error.Is(err, errors.ErrUserNotFound) {
			errors.UnauthorizedResponse(ctx, err.Error())
		} else {
			errors.InternalServerErrorResponse(ctx, err.Error())
		}
		return
	}

	// set refresh token (30 days)
	ctx.SetCookie(
		"refresh_token",
		loginResp.RefreshToken,
		int(jwttypes.RefreshTokenDuration),
		jwttypes.CookiePath,
		"",
		false,
		true,
	)

	// set access token (30 mins)
	ctx.SetCookie(
		"jwt",
		loginResp.AccessToken,
		int(jwttypes.AccessTokenDuration),
		jwttypes.CookiePath,
		"",
		false,
		true,
	)

	responses.JSONSuccess(ctx, "login successful")
}

func (h *AuthHandler) Refresh(ctx *gin.Context) {
	oldRefreshToken, err := ctx.Cookie("refresh_token")
	if err != nil || oldRefreshToken == "" {
		errors.UnauthorizedResponse(ctx, "missing refresh token")
		return
	}

	tokenPair, err := h.authService.RefreshToken(ctx, oldRefreshToken)
	if err != nil {
		if error.Is(err, errors.ErrUserNotFound) || error.Is(err, errors.ErrInvalidToken) || error.Is(err, errors.ErrInvalidTokenType) {
			errors.ConflictResponse(ctx, err.Error())
		} else {
			errors.InternalServerErrorResponse(ctx, err.Error())
		}
		return
	}

	ctx.SetCookie("jwt", tokenPair.AccessToken, int(jwttypes.AccessTokenDuration), jwttypes.CookiePath, "", false, true)
	ctx.SetCookie("refresh_token", tokenPair.RefreshToken, int(jwttypes.RefreshTokenDuration), jwttypes.CookiePath, "", false, true)
	responses.JSONSuccess(ctx, "token refreshed")
}

func (h *AuthHandler) Logout(ctx *gin.Context) {
	ctx.SetCookie("jwt", "", -1, jwttypes.CookiePath, "", false, true)
	ctx.SetCookie("refresh_token", "", -1, jwttypes.CookiePath, "", false, true)
	responses.JSONSuccess(ctx, "logged out")
}
