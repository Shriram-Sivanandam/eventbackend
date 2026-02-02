package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/Shriram-Sivanandam/eventbackend/internal/auth"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuthHandler struct {
	DB *pgxpool.Pool
}

func (h *AuthHandler) RequestOTP(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}

	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		http.Error(w, "Invalid json", http.StatusBadRequest)
	}

	if body.Email == "" {
		http.Error(w, "Email required", http.StatusBadRequest)
		return
	}

	otp := auth.GenerateOTP()
	hash, err := auth.Hash(otp)
	if err != nil {
		http.Error(w, "Error in hashing", http.StatusInternalServerError)
		return
	}

	_ , err = h.DB.Exec(r.Context(),
	`INSERT INTO auth_otps(email, otp_hash, expires_at)
	VALUES($1, $2, NOW()+INTERVAL '5 minutes')`,
	body.Email, hash)

	if err != nil {
		http.Error(w, "Inserting OTP failed", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) VerifyOTP(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
		OTP string `json:"otp"`
	}

	json.NewDecoder(r.Body).Decode(&body)

	var hash string
	err := h.DB.QueryRow(r.Context(),
	`SELECT otp_hash FROM auth_otps
	WHERE email = $1 AND expires_at > NOW()
	ORDER BY created_at DESC LIMIT 1`,
	body.Email).Scan(&hash)

	if err != nil || !auth.Compare(hash, body.OTP) {
		http.Error(w, "invalid OTP", http.StatusBadRequest)
		return
	}

	var userID string
	err = h.DB.QueryRow(r.Context(),
	`INSERT INTO users(email,name)
	VALUES($1,$1)
	ON CONFLICT(email) DO UPDATE SET email=EXCLUDED.email
	RETURNING id`,
	body.Email).Scan(&userID)

	token, err := auth.CreateJWT(userID)
	if err != nil {
		http.Error(w, "Error occured while creating JWT", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"token": token,
	})
}