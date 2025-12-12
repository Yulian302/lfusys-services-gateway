package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"log"
	"net/http"
	"strings"
	"time"

	common "github.com/Yulian302/lfusys-services-commons"
	jwttypes "github.com/Yulian302/lfusys-services-commons/jwt"
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

func (h *AuthHandler) Register(ctx *gin.Context) {
	var req types.RegisterUser
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}

	var user types.User
	user.Email = req.Email
	user.Name = req.Name
	hasher := sha256.New()
	hasher.Write([]byte(req.Password))
	user.Password = hex.EncodeToString(hasher.Sum(nil))
	uuidVal := uuid.New()
	user.ID = uuidVal.String()

	userItem, err := attributevalue.MarshalMap(user)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "could not marshal user, bad fields",
		})
		return
	}

	_, err = h.store.Client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(h.store.TableName),
		Item:                userItem,
		ConditionExpression: aws.String("attribute_not_exists(email)"),
	})
	if err != nil {
		if strings.Contains(err.Error(), "ConditionalCheckFailedException") {
			ctx.JSON(http.StatusConflict, gin.H{"error": "user already exists"})
			return
		}
		log.Printf("Couldn't add item to table. Here's why: %v\n", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "could not create user",
		})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"message": "created",
	})
}

func (h *AuthHandler) Login(ctx *gin.Context) {
	var loginUser types.LoginUser

	if err := ctx.ShouldBindJSON(&loginUser); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid input",
		})
		return
	}
	res, err := h.store.Client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(h.store.TableName),
		Key: map[string]dynamoTypes.AttributeValue{
			"email": &dynamoTypes.AttributeValueMemberS{Value: loginUser.Email},
		},
	})
	if err != nil || res.Item == nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{
			"error": "invalid credentials",
		})
		return
	}

	var user types.User

	if err = attributevalue.UnmarshalMap(res.Item, &user); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "could not parse user data"})
		return
	}

	// verify password
	hasher := sha256.New()
	hasher.Write([]byte(loginUser.Password))
	hashedPassword := hex.EncodeToString(hasher.Sum(nil))

	if user.Password != hashedPassword {
		ctx.JSON(http.StatusUnauthorized, gin.H{
			"error": "invalid credentials",
		})
		return
	}

	// access token
	accessClaims := jwttypes.JWTClaims{
		Issuer:    "lfusys",
		Subject:   user.ID,
		ExpiresAt: time.Now().Add(30 * time.Minute).Unix(),
		IssuedAt:  time.Now().Unix(),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)

	s, err := t.SignedString([]byte(h.config.JWTConfig.SECRET_KEY))
	if err != nil {
		log.Printf("could not sign JWT token: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "token creation failed",
		})
		return
	}

	// refresh token
	refreshTokenCookie, err := ctx.Cookie("refresh_token")
	var refs string
	if err != nil || refreshTokenCookie == "" {
		refreshClaims := jwttypes.JWTClaims{
			Issuer:    "lfusys",
			Subject:   user.ID,
			ExpiresAt: time.Now().Add(30 * 24 * time.Hour).Unix(),
			IssuedAt:  time.Now().Unix(),
			Type:      "refresh",
		}
		ref := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
		refs, err = ref.SignedString([]byte(h.config.JWTConfig.SECRET_KEY))
		if err != nil {
			log.Printf("could not sign refresh token: %v", err)
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"error": "token creation failed",
			})
			return
		}
		// set refresh token (30 days)
		ctx.SetCookie(
			"refresh_token",
			refs,
			30*24*60*60,
			"/",
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
		"/",
		"",
		h.config.Env != "DEV",
		true,
	)

	ctx.JSON(http.StatusOK, gin.H{
		"message": "login successful",
	})
}

func (h *AuthHandler) Refresh(ctx *gin.Context) {
	refreshToken, err := ctx.Cookie("refresh_token")
	if err != nil || refreshToken == "" {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "no refresh token"})
		return
	}

	token, err := jwt.ParseWithClaims(refreshToken, &jwttypes.JWTClaims{}, func(t *jwt.Token) (any, error) {
		return []byte(h.config.JWTConfig.SECRET_KEY), nil
	})
	if err != nil || !token.Valid {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh token"})
		return
	}

	claims := token.Claims.(*jwttypes.JWTClaims)
	if claims.Type != "refresh" {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token type"})
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
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "token creation failed"})
		return
	}

	ctx.SetCookie("jwt", s, 30*60, "/", "", h.config.Env != "DEV", true)
	ctx.JSON(http.StatusOK, gin.H{"message": "token refreshed"})
}

func (h *AuthHandler) Logout(ctx *gin.Context) {
	ctx.SetCookie("jwt", "", -1, "/", "", h.config.Env != "DEV", true)
	ctx.SetCookie("refresh_token", "", -1, "/", "", h.config.Env != "DEV", true)
	ctx.JSON(http.StatusOK, gin.H{"message": "logged out"})
}
