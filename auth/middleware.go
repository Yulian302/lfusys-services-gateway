package auth

import (
	"github.com/Yulian302/lfusys-services-commons/errors"
	jwttypes "github.com/Yulian302/lfusys-services-commons/jwt"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func JWTMiddleware(secretKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie("jwt")
		if err != nil || token == "" {
			errors.UnauthorizedResponse(c, "unauthorized")
			c.Abort()
			return
		}

		parsedToken, err := jwt.ParseWithClaims(token, &jwttypes.JWTClaims{}, func(t *jwt.Token) (any, error) {
			return []byte(secretKey), nil
		})
		if err != nil || !parsedToken.Valid {
			refresh, _ := c.Cookie("refresh_token")
			if refresh != "" {
				errors.UnauthorizedResponse(c, "token_expired")
			} else {
				errors.UnauthorizedResponse(c, "invalid_token")
			}
			c.Abort()
			return
		}

		claims := parsedToken.Claims.(*jwttypes.JWTClaims)
		if claims.Type != "access" {
			errors.UnauthorizedResponse(c, "invalid token type")
			c.Abort()
			return
		}

		c.Set("email", claims.Subject)
		c.Next()
	}
}
