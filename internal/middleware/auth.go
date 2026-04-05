package middleware

import (
	"PingGoat/internal/httputil"
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// contextKey is an unexported type for context keys in this package.
// Using a custom type prevents collisions with keys defined in other packages.
type contextKey string

const UserIDKey contextKey = "userID"

// JWTAuth returns middleware that validates Bearer tokens in the Authorization header.
// jwtSecret must be the same secret used to sign tokens in your login handler.
func JWTAuth(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authTokenHeader := r.Header.Get("Authorization")
			if authTokenHeader == "" {
				httputil.RespondWithError(w, http.StatusUnauthorized, "Missing Authorization header")
				return
			}

			if !strings.HasPrefix(authTokenHeader, "Bearer ") {
				httputil.RespondWithError(w, http.StatusUnauthorized, "Invalid Authorization header format")
				return
			}

			authTokenHeader = strings.TrimPrefix(authTokenHeader, "Bearer ")
			authTokenHeader = strings.TrimSpace(authTokenHeader)

			token, err := jwt.ParseWithClaims(authTokenHeader, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
				// verified the algorithm used is the same
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				return []byte(jwtSecret), nil
			})

			if err != nil {
				httputil.RespondWithError(w, http.StatusUnauthorized, "Invalid token")
				return
			}

			claims, ok := token.Claims.(*jwt.RegisteredClaims)
			if !ok || claims.Subject == "" {
				httputil.RespondWithError(w, http.StatusUnauthorized, "Invalid token")
				return
			}

			userID := claims.Subject
			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
