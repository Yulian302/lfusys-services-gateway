package auth

import (
	"github.com/Yulian302/lfusys-services-commons/errors"
	jwttypes "github.com/Yulian302/lfusys-services-commons/jwt"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func JWTMiddleware(secretKey string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		token, err := ctx.Cookie("jwt")
		if err != nil || token == "" {
			errors.Unauthorized(ctx, "unauthorized")
			ctx.Abort()
			return
		}

		parsedToken, err := jwt.ParseWithClaims(token, &jwttypes.JWTClaims{}, func(t *jwt.Token) (any, error) {
			return []byte(secretKey), nil
		})
		if err != nil {
			errors.Unauthorized(ctx, "invalid token")
			ctx.Abort()
			return
		}

		claims := parsedToken.Claims.(*jwttypes.JWTClaims)
		ctx.Set("email", claims.Subject)
		ctx.Next()
	}
}
