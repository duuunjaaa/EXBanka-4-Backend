package middleware

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// GetUserIDFromToken extracts the user_id claim from the Bearer token in the request.
func GetUserIDFromToken(c *gin.Context) (int64, error) {
	header := c.GetHeader("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return 0, fmt.Errorf("missing token")
	}
	tokenStr := strings.TrimPrefix(header, "Bearer ")

	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(jwtSecret), nil
	})
	if err != nil || !token.Valid {
		return 0, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, fmt.Errorf("invalid token claims")
	}

	switch v := claims["user_id"].(type) {
	case float64:
		return int64(v), nil
	case int64:
		return v, nil
	default:
		return 0, fmt.Errorf("user_id not found in token")
	}
}

var jwtSecret = os.Getenv("JWT_SECRET")

// GetCallerRoleFromToken returns "CLIENT" for client tokens, "EMPLOYEE" for employee tokens.
func GetCallerRoleFromToken(c *gin.Context) string {
	header := c.GetHeader("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return ""
	}
	tokenStr := strings.TrimPrefix(header, "Bearer ")
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(jwtSecret), nil
	})
	if err != nil || !token.Valid {
		return ""
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return ""
	}
	if role, ok := claims["role"].(string); ok {
		return role
	}
	if _, hasDozv := claims["dozvole"]; hasDozv {
		return "EMPLOYEE"
	}
	return ""
}

// CallerHasPermission reports whether the authenticated caller has the given permission
// in their dozvole array (or is ADMIN, which satisfies any permission check).
func CallerHasPermission(c *gin.Context, perm string) bool {
	header := c.GetHeader("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return false
	}
	tokenStr := strings.TrimPrefix(header, "Bearer ")
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(jwtSecret), nil
	})
	if err != nil || !token.Valid {
		return false
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return false
	}
	target := strings.ToUpper(perm)
	dozvole, _ := claims["dozvole"].([]interface{})
	for _, d := range dozvole {
		s := strings.ToUpper(fmt.Sprintf("%v", d))
		if s == "ADMIN" || s == target {
			return true
		}
	}
	return false
}

// RequireRole returns a Gin middleware that validates the JWT and checks that
// the caller has at least one of the given roles (or ADMIN, which bypasses all role checks).
func RequireRole(roles ...string) gin.HandlerFunc {
	required := make([]string, len(roles))
	for i, r := range roles {
		required[i] = strings.ToUpper(r)
	}
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}
		tokenStr := strings.TrimPrefix(header, "Bearer ")

		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(jwtSecret), nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token claims"})
			return
		}

		if claims["type"] != "access" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token type"})
			return
		}

		dozvole, _ := claims["dozvole"].([]interface{})
		for _, d := range dozvole {
			s := strings.ToUpper(fmt.Sprintf("%v", d))
			if s == "ADMIN" {
				c.Next()
				return
			}
			for _, r := range required {
				if s == r {
					c.Next()
					return
				}
			}
		}

		// CLIENT tokens use a "role" claim instead of "dozvole".
		if roleStr, ok := claims["role"].(string); ok {
			roleUpper := strings.ToUpper(roleStr)
			for _, r := range required {
				if roleUpper == r {
					c.Next()
					return
				}
			}
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
	}
}
