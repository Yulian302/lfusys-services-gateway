package auth

import (
	"log"
	"net/http"
	"strings"
	"time"

	common "github.com/Yulian302/lfusys-services-commons"
	"github.com/Yulian302/lfusys-services-commons/crypt"
	"github.com/Yulian302/lfusys-services-commons/errors"
	jwttypes "github.com/Yulian302/lfusys-services-commons/jwt"
	"github.com/Yulian302/lfusys-services-commons/responses"
	"github.com/Yulian302/lfusys-services-gateway/auth/types"
	"github.com/Yulian302/lfusys-services-gateway/store"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/google/uuid"
)

const (
	AccessTokenDuration  = 30 * time.Minute
	RefreshTokenDuration = 30 * 24 * time.Hour
	CookiePath           = "/"
)

type AuthHandler struct {
	store  store.UserStore
	config *common.Config
}

func NewAuthHandler(store store.UserStore, config *common.Config) *AuthHandler {
	return &AuthHandler{
		store:  store,
		config: config,
	}
}

func (h *AuthHandler) Me(ctx *gin.Context) {
	token, err := ctx.Cookie("jwt")
	if err != nil || token == "" {
		errors.Unauthorized(ctx, "unauthorized")
		return
	}

	parsedToken, err := jwt.ParseWithClaims(token, &jwttypes.JWTClaims{}, func(t *jwt.Token) (any, error) {
		return []byte(h.config.JWTConfig.SECRET_KEY), nil
	})

	if err != nil || !parsedToken.Valid {
		errors.Unauthorized(ctx, "invalid token")
		return
	}

	claims := parsedToken.Claims.(*jwttypes.JWTClaims)

	user, err := h.store.GetByEmail(ctx, claims.Subject)
	if err != nil {
		log.Printf("could not get user record: %v", err)
		errors.InternalServerError(ctx, "could not get user data")
		return
	}

	responses.JSONData(ctx, http.StatusOK, types.MeResponse{
		Email:         claims.Subject,
		Name:          user.Name,
		Authenticated: true,
	})

}

func (h *AuthHandler) Register(ctx *gin.Context) {
	var req types.RegisterUser
	if err := ctx.ShouldBindJSON(&req); err != nil {
		errors.BadRequestError(ctx, err.Error())
		return
	}

	var user types.User
	user.Email = req.Email
	user.Name = req.Name
	user.Password, user.Salt = crypt.HashPasswordWithSalt(req.Password)
	uuidVal := uuid.New()
	user.ID = uuidVal.String()

	err := h.store.Create(ctx, user)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			errors.JSONError(ctx, http.StatusConflict, "user already exists")
			return
		}
		errors.InternalServerError(ctx, "could not create user")
		return
	}

	responses.JSONCreated(ctx, "created")
}

func (h *AuthHandler) Login(ctx *gin.Context) {
	var loginUser types.LoginUser

	if err := ctx.ShouldBindJSON(&loginUser); err != nil {
		errors.BadRequestError(ctx, err.Error())
		return
	}

	user, err := h.store.GetByEmail(ctx, loginUser.Email)
	if err != nil {
		errors.Unauthorized(ctx, "invalid credentials")
		return
	}

	if !crypt.VerifyPasswordWithSalt(loginUser.Password, user.Password, user.Salt) {
		errors.Unauthorized(ctx, "invalid credentials")
		return
	}

	// access token
	accessJti := uuid.New().String()
	accessClaims := jwttypes.JWTClaims{
		Issuer:    "lfusys",
		Subject:   user.Email,
		ExpiresAt: time.Now().Add(AccessTokenDuration).Unix(),
		IssuedAt:  time.Now().Unix(),
		Type:      "access",
		JTI:       accessJti,
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)

	s, err := t.SignedString([]byte(h.config.JWTConfig.SECRET_KEY))
	if err != nil {
		log.Printf("could not sign JWT token: %v", err)
		errors.InternalServerError(ctx, "token creation failed")
		return
	}

	// refresh token
	refreshJti := uuid.New().String()
	refreshClaims := jwttypes.JWTClaims{
		Issuer:    "lfusys",
		Subject:   user.Email,
		ExpiresAt: time.Now().Add(RefreshTokenDuration).Unix(),
		IssuedAt:  time.Now().Unix(),
		Type:      "refresh",
		JTI:       refreshJti,
	}
	ref := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refs, err := ref.SignedString([]byte(h.config.JWTConfig.REFRESH_SECRET_KEY))
	if err != nil {
		log.Printf("could not sign refresh token: %v", err)
		errors.InternalServerError(ctx, "token creation failed")
		return
	}
	// set refresh token (30 days)
	ctx.SetCookie(
		"refresh_token",
		refs,
		int(RefreshTokenDuration),
		CookiePath,
		"",
		h.config.Env != "DEV",
		true,
	)

	// set jwt access token (30 mins)
	ctx.SetCookie(
		"jwt",
		s,
		int(AccessTokenDuration),
		CookiePath,
		"",
		h.config.Env != "DEV",
		true,
	)

	responses.JSONSuccess(ctx, "login successful")
}

func (h *AuthHandler) Refresh(ctx *gin.Context) {
	oldRefreshToken, err := ctx.Cookie("refresh_token")
	if err != nil || oldRefreshToken == "" {
		errors.Unauthorized(ctx, "missing refresh token")
		return
	}

	token, err := jwt.ParseWithClaims(oldRefreshToken, &jwttypes.JWTClaims{}, func(t *jwt.Token) (any, error) {
		return []byte(h.config.JWTConfig.REFRESH_SECRET_KEY), nil
	})
	if err != nil || !token.Valid {
		errors.Unauthorized(ctx, "invalid refresh token")
		return
	}

	claims := token.Claims.(*jwttypes.JWTClaims)
	if claims.Type != "refresh" {
		errors.Unauthorized(ctx, "invalid token type")
		return
	}

	user, err := h.store.GetByEmail(ctx, claims.Subject)
	if err != nil || user == nil {
		errors.Unauthorized(ctx, "user not found")
		return
	}

	accessJti := uuid.New().String()
	accessClaims := jwttypes.JWTClaims{
		Issuer:    "lfusys",
		Subject:   claims.Subject,
		ExpiresAt: time.Now().Add(AccessTokenDuration).Unix(),
		IssuedAt:  time.Now().Unix(),
		Type:      "access",
		JTI:       accessJti,
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	s, err := t.SignedString([]byte(h.config.JWTConfig.SECRET_KEY))
	if err != nil {
		errors.InternalServerError(ctx, "token creation failed")
		return
	}

	refreshJti := uuid.New().String()
	newRefClaims := jwttypes.JWTClaims{
		Issuer:    "lfusys",
		Subject:   claims.Subject,
		ExpiresAt: time.Now().Add(RefreshTokenDuration).Unix(),
		IssuedAt:  time.Now().Unix(),
		Type:      "refresh",
		JTI:       refreshJti,
	}
	newRefToken := jwt.NewWithClaims(jwt.SigningMethodHS256, newRefClaims)
	refs, err := newRefToken.SignedString([]byte(h.config.JWTConfig.REFRESH_SECRET_KEY))
	if err != nil {
		errors.InternalServerError(ctx, "refresh token creation failed")
		return
	}

	ctx.SetCookie("jwt", s, int(AccessTokenDuration), CookiePath, "", h.config.Env != "DEV", true)
	ctx.SetCookie("refresh_token", refs, int(RefreshTokenDuration), CookiePath, "", h.config.Env != "DEV", true)
	responses.JSONSuccess(ctx, "token refreshed")
}

func (h *AuthHandler) Logout(ctx *gin.Context) {
	ctx.SetCookie("jwt", "", -1, CookiePath, "", h.config.Env != "DEV", true)
	ctx.SetCookie("refresh_token", "", -1, CookiePath, "", h.config.Env != "DEV", true)
	responses.JSONSuccess(ctx, "logged out")
}
