package routers

import (
	"crypto/sha256"
	"encoding/hex"
	"log"
	"net/http"
	"strings"

	"github.com/Yulian302/lfusys-services-gateway/auth/types"
	"github.com/Yulian302/lfusys-services-gateway/store"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func AuthRoutes(route *gin.Engine, store *store.DynamoDbStore) {
	auth := route.Group("/auth")

	auth.POST("/register", func(ctx *gin.Context) {
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

		_, err = store.Client.PutItem(ctx, &dynamodb.PutItemInput{
			TableName:           aws.String(store.TableName),
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

	})
}
