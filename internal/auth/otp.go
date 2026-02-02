package auth

import (
	"crypto/rand"

	"fmt"
)

func GenerateOTP() string {
	b := make([]byte, 3)
	rand.Read(b)
	return fmt.Sprintf("%06d", int(b[0])%100000)
}