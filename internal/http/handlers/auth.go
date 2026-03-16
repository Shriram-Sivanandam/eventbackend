package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/Shriram-Sivanandam/eventbackend/internal/auth"
	"github.com/Shriram-Sivanandam/eventbackend/internal/db"
	"github.com/Shriram-Sivanandam/eventbackend/internal/email"
	"github.com/Shriram-Sivanandam/eventbackend/internal/http/middleware"
	"github.com/google/uuid"
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
		return
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

	if err := email.SendOTP(body.Email, otp); err != nil {
		http.Error(w, "Failed to send OTP email", http.StatusInternalServerError)
		fmt.Println("Failed to send OTP email:", err)
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

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userIDVal := r.Context().Value(middleware.UserIDKey);
	if userIDVal == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	userID := userIDVal.(string)

	var user struct {
		ID string `json:"id"`
		Email string `json:"email"`
		Name string `json:"name"`
	}

	err := h.DB.QueryRow(r.Context(),
		`SELECT id,email,name FROM users WHERE id=$1`,
		userID,
	).Scan(&user.ID, &user.Email, &user.Name)

	if err != nil {
		http.Error(w, "Error occured while selecting user details", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(user)
}

// PATCH /auth/me
var validGenders = map[string]bool{
	"male":              true,
	"female":            true,
	"non_binary":        true,
	"prefer_not_to_say": true,
}

func (h *AuthHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	callerID, err := uuid.Parse(r.Context().Value(middleware.UserIDKey).(string))
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
 
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}
 
	params := db.UpdateProfileParams{
		Name:      stringPtrOrNil(r.FormValue("name")),
		Phone:     stringPtrOrNil(r.FormValue("phone")),
		Bio:       stringPtrOrNil(r.FormValue("bio")),
		City:      stringPtrOrNil(r.FormValue("city")),
	}
 
	if g := r.FormValue("gender"); g != "" {
		if !validGenders[g] {
			http.Error(w, "invalid gender value", http.StatusBadRequest)
			return
		}
		params.Gender = &g
	}
 
	if a := r.FormValue("age"); a != "" {
		age, err := strconv.Atoi(a)
		if err != nil || age < 13 || age > 120 {
			http.Error(w, "age must be a number between 13 and 120", http.StatusBadRequest)
			return
		}
		params.Age = &age
	}
 
	file, header, err := r.FormFile("avatar")
	if err == nil {
		defer file.Close()
		filename := uuid.New().String() + filepath.Ext(header.Filename)
		path := "./uploads/" + filename
		out, err := os.Create(path)
		if err != nil {
			http.Error(w, "error saving avatar", http.StatusInternalServerError)
			return
		}
		defer out.Close()
		io.Copy(out, file)
		url := "/uploads/" + filename
		params.AvatarURL = &url
	}
 
	if err := db.UpdateProfile(r.Context(), h.DB, callerID, params); err != nil {
		http.Error(w, "failed to update profile", http.StatusInternalServerError)
		return
	}
 
	w.WriteHeader(http.StatusNoContent)
}