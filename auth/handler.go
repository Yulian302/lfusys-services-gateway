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
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamoTypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
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
	store  *store.DynamoDbStore
	config *common.Config
}

func NewAuthHandler(store *store.DynamoDbStore, config *common.Config) *AuthHandler {
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

	res, err := h.store.Client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &h.store.UsersTableName,
		Key: map[string]dynamoTypes.AttributeValue{
			"email": &dynamoTypes.AttributeValueMemberS{Value: claims.Subject},
		},
	})
	if err != nil {
		log.Printf("could not get user record: %v", err)
		errors.InternalServerError(ctx, "could not get user data")
		return
	}

	var user types.User
	if err = attributevalue.UnmarshalMap(res.Item, &user); err != nil {
		log.Printf("could not unmarshal user: %v", err)
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

	userItem, err := attributevalue.MarshalMap(user)
	if err != nil {
		errors.BadRequestError(ctx, err.Error())
		return
	}

	_, err = h.store.Client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(h.store.UsersTableName),
		Item:                userItem,
		ConditionExpression: aws.String("attribute_not_exists(email)"),
	})
	if err != nil {
		if strings.Contains(err.Error(), "ConditionalCheckFailedException") {
			errors.JSONError(ctx, http.StatusConflict, "user already exists")
			return
		}
		log.Printf("Couldn't add item to table. Here's why: %v\n", err)
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
	res, err := h.store.Client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(h.store.UsersTableName),
		Key: map[string]dynamoTypes.AttributeValue{
			"email": &dynamoTypes.AttributeValueMemberS{Value: loginUser.Email},
		},
	})
	if err != nil || res.Item == nil {
		errors.Unauthorized(ctx, "invalid credentials")
		return
	}

	var user types.User

	if err = attributevalue.UnmarshalMap(res.Item, &user); err != nil {
		errors.InternalServerError(ctx, "could not parse user data")
		return
	}

	if !crypt.VerifyPasswordWithSalt(loginUser.Password, user.Password, user.Salt) {
		errors.Unauthorized(ctx, "invalid credentials")
		return
	}

	// access token
	accessClaims := jwttypes.JWTClaims{
		Issuer:    "lfusys",
		Subject:   user.Email,
		ExpiresAt: time.Now().Add(AccessTokenDuration).Unix(),
		IssuedAt:  time.Now().Unix(),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)

	s, err := t.SignedString([]byte(h.config.JWTConfig.SECRET_KEY))
	if err != nil {
		log.Printf("could not sign JWT token: %v", err)
		errors.InternalServerError(ctx, "token creation failed")
		return
	}

	// refresh token
	refreshTokenCookie, err := ctx.Cookie("refresh_token")
	var refs string
	if err != nil || refreshTokenCookie == "" {
		refreshClaims := jwttypes.JWTClaims{
			Issuer:    "lfusys",
			Subject:   user.Email,
			ExpiresAt: time.Now().Add(RefreshTokenDuration).Unix(),
			IssuedAt:  time.Now().Unix(),
			Type:      "refresh",
		}
		ref := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
		refs, err = ref.SignedString([]byte(h.config.JWTConfig.SECRET_KEY))
		if err != nil {
			log.Printf("could not sign refresh token: %v", err)
			errors.InternalServerError(ctx, "token creation failed")
			return
		}
		// set refresh token (30 days)
		ctx.SetCookie(
			"refresh_token",
			refs,
			30*24*60*60,
			CookiePath,
			"",
			h.config.Env != "DEV",
			true,
		)
	}

	// set jwt access token (30 mins)
	ctx.SetCookie(
		"jwt",
		s,
		30*60,
		CookiePath,
		"",
		h.config.Env != "DEV",
		true,
	)

	responses.JSONSuccess(ctx, "login successful")
}

func (h *AuthHandler) Refresh(ctx *gin.Context) {
	refreshToken, err := ctx.Cookie("refresh_token")
	if err != nil || refreshToken == "" {
		errors.Unauthorized(ctx, "missing refresh token")
		return
	}

	token, err := jwt.ParseWithClaims(refreshToken, &jwttypes.JWTClaims{}, func(t *jwt.Token) (any, error) {
		return []byte(h.config.JWTConfig.SECRET_KEY), nil
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

	accessClaims := jwttypes.JWTClaims{
		Issuer:    "lfusys",
		Subject:   claims.Subject,
		ExpiresAt: time.Now().Add(30 * time.Minute).Unix(),
		IssuedAt:  time.Now().Unix(),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	s, err := t.SignedString([]byte(h.config.JWTConfig.SECRET_KEY))
	if err != nil {
		errors.InternalServerError(ctx, "token creation failed")
		return
	}

	ctx.SetCookie("jwt", s, 30*60, CookiePath, "", h.config.Env != "DEV", true)
	responses.JSONSuccess(ctx, "token refreshed")
}

func (h *AuthHandler) Logout(ctx *gin.Context) {
	ctx.SetCookie("jwt", "", -1, CookiePath, "", h.config.Env != "DEV", true)
	ctx.SetCookie("refresh_token", "", -1, CookiePath, "", h.config.Env != "DEV", true)
	responses.JSONSuccess(ctx, "logged out")
}
