package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"log"
	"net/http"
	"strings"

	common "github.com/Yulian302/lfusys-services-commons"
	"github.com/Yulian302/lfusys-services-gateway/auth/types"
	"github.com/Yulian302/lfusys-services-gateway/store"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamoTypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/gin-gonic/gin"
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

	ctx.SetCookie(
		"user_id",
		user.ID,
		7*24*60*60,
		"/",
		"",
		h.config.Env != "DEV",
		true,
	)

	ctx.JSON(http.StatusOK, gin.H{
		"message": "login successful",
		"user_id": user.ID,
	})
}
