package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if h == "" {
			http.Error(w, "missing token", http.StatusBadRequest)
			return
		}

		tokenStr := strings.Replace(h, "Bearer ", "", 1)

		_, err := jwt.Parse(tokenStr, func(t *jwt.Token)(interface{}, error) {
			return []byte(os.Getenv("JWT_SECRET")), nil
		})

		if err != nil {
			http.Error(w, "not authorized", http.StatusBadRequest)
			return
		}

		next.ServeHTTP(w, r)
	})
}