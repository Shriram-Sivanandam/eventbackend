package middleware

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const UserIDKey = contextKey("user_id")

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if h == "" {
			http.Error(w, "missing token", http.StatusBadRequest)
			return
		}

		tokenStr := strings.Replace(h, "Bearer ", "", 1)

		token, err := jwt.Parse(tokenStr, func(t *jwt.Token)(interface{}, error) {
			return []byte(os.Getenv("JWT_SECRET")), nil
		})

		if err != nil {
			http.Error(w, "not authorized", http.StatusBadRequest)
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			http.Error(w, "invalid token claims", http.StatusUnauthorized)
			return
		}
		userIDVal, ok := claims["user_id"]
		if !ok {
			http.Error(w, "token missing user_id", http.StatusUnauthorized)
			return
		}

		userID, ok := userIDVal.(string)
		if !ok {
			http.Error(w, "invalid user_id in token", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), UserIDKey, userID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}