package auth

import (
	"github.com/Yulian302/lfusys-services-commons/errors"
	jwttypes "github.com/Yulian302/lfusys-services-commons/jwt"
	logger "github.com/Yulian302/lfusys-services-commons/logging"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func JWTMiddleware(secretKey string, l logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie("jwt")
		if err != nil || token == "" {
			l.Warn("jwt authentication failed",
				"ip", c.ClientIP(),
				"reason", "missing_token",
			)
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
				l.Warn("jwt authentication failed",
					"ip", c.ClientIP(),
					"reason", "token_expired",
				)
				errors.UnauthorizedResponse(c, "token_expired")
			} else {
				l.Warn("jwt authentication failed",
					"ip", c.ClientIP(),
					"reason", "invalid_token",
				)
				errors.UnauthorizedResponse(c, "invalid_token")
			}
			c.Abort()
			return
		}

		claims := parsedToken.Claims.(*jwttypes.JWTClaims)
		if claims.Type != "access" {
			l.Warn("jwt authentication failed",
				"ip", c.ClientIP(),
				"email", claims.Subject,
				"reason", "invalid_token_type",
			)
			errors.UnauthorizedResponse(c, "invalid token type")
			c.Abort()
			return
		}

		c.Set("email", claims.Subject)
		c.Next()
	}
}
