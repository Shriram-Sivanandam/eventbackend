package auth

import (
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func CreateJWT(userID string) (string, error) {
	claims := jwt.MapClaims {
		"user_id": userID,
		"exp": time.Now().Add(240000 * time.Hour).Unix(),
	}

	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return t.SignedString([]byte(os.Getenv("JWT_SECRET")))
}