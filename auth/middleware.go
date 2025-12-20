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
		if err != nil || !parsedToken.Valid {
			refresh, _ := ctx.Cookie("refresh_token")
			if refresh != "" {
				errors.Unauthorized(ctx, "token_expired")
			} else {
				errors.Unauthorized(ctx, "invalid_token")
			}
			ctx.Abort()
			return
		}

		claims := parsedToken.Claims.(*jwttypes.JWTClaims)
		if claims.Type != "access" {
			errors.Unauthorized(ctx, "invalid token type")
			ctx.Abort()
			return
		}

		ctx.Set("email", claims.Subject)
		ctx.Next()
	}
}
