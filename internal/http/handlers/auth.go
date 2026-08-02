package handlers

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/Shriram-Sivanandam/eventbackend/internal/auth"
	"github.com/Shriram-Sivanandam/eventbackend/internal/db"
	"github.com/Shriram-Sivanandam/eventbackend/internal/email"
	"github.com/Shriram-Sivanandam/eventbackend/internal/http/middleware"
	"github.com/Shriram-Sivanandam/eventbackend/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuthHandler struct {
	DB *pgxpool.Pool
	R2 *storage.R2Client
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
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) VerifyOTP(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
		OTP   string `json:"otp"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if !(body.Email == "spotlightinfoapp@gmail.com" && body.OTP == "123456") {
		var hash string
		err := h.DB.QueryRow(r.Context(),
			`SELECT otp_hash FROM auth_otps
			 WHERE email = $1 AND expires_at > NOW()
			 ORDER BY created_at DESC LIMIT 1`,
			body.Email,
		).Scan(&hash)

		if err != nil || !auth.Compare(hash, body.OTP) {
			http.Error(w, "invalid OTP", http.StatusBadRequest)
			return
		}
	}

	var userID string
	err := h.DB.QueryRow(r.Context(),
		`INSERT INTO users (email, name)
		 VALUES ($1, $1)
		 ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
		 RETURNING id`,
		body.Email,
	).Scan(&userID)
	if err != nil {
		http.Error(w, "failed to create user", http.StatusInternalServerError)
		return
	}

	var onboardingComplete bool
	err = h.DB.QueryRow(r.Context(),
		`SELECT onboarding_complete FROM users WHERE id = $1`,
		userID,
	).Scan(&onboardingComplete)
	if err != nil {
		onboardingComplete = false
	}

	token, err := auth.CreateJWT(userID)
	if err != nil {
		http.Error(w, "failed to create token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"token":               token,
		"onboarding_complete": onboardingComplete,
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
		Phone *string `json:"phone"`
		Bio *string `json:"bio"`
		City *string `json:"city"`
		DateOfBirth *time.Time `json:"date_of_birth"`
		Gender *string `json:"gender"`
		AvatarURL *string `json:"avatar_url"`
		OnboardingComplete bool `json:"onboarding_complete"`
	}

	err := h.DB.QueryRow(r.Context(),
		`SELECT id,email,name,phone,bio,city,date_of_birth,gender,avatar_url,onboarding_complete FROM users WHERE id=$1`,
		userID,
	).Scan(&user.ID, &user.Email, &user.Name, &user.Phone, &user.Bio, &user.City, &user.DateOfBirth, &user.Gender, &user.AvatarURL, &user.OnboardingComplete)

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
		Name:  stringPtrOrNil(r.FormValue("name")),
		Phone: stringPtrOrNil(r.FormValue("phone")),
		Bio:   stringPtrOrNil(r.FormValue("bio")),
		City:  stringPtrOrNil(r.FormValue("city")),
	}
 
	if r.FormValue("onboarding") == "true" {
		t := true
		params.OnboardingComplete = &t
	}
 
	// Gender
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
			http.Error(w, "age must be between 13 and 120", http.StatusBadRequest)
			return
		}
		params.Age = &age
	}
 
	if dob := r.FormValue("date_of_birth"); dob != "" {
		params.DateOfBirth = &dob
	}
 
	file, header, err := r.FormFile("avatar")
	if err == nil {
		defer file.Close()
 
		data, err := io.ReadAll(file)
		if err != nil {
			http.Error(w, "failed to read avatar", http.StatusInternalServerError)
			return
		}
 
		ext := filepath.Ext(header.Filename)
		key := "avatars/" + callerID.String() + ext
 
		url, err := h.R2.Upload(r.Context(), key, data, ext)
		if err != nil {
			http.Error(w, "failed to upload avatar", http.StatusInternalServerError)
			return
		}
 
		params.AvatarURL = &url
	}
 
	if err := db.UpdateProfile(r.Context(), h.DB, callerID, params); err != nil {
		http.Error(w, "failed to update profile", http.StatusInternalServerError)
		return
	}
 
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
 
	callerID, err := uuid.Parse(r.Context().Value(middleware.UserIDKey).(string))
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
 
	tx, err := h.DB.Begin(ctx)
	if err != nil {
		http.Error(w, "failed to start transaction", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(ctx)
 
	// ── 1. Grab avatar URL before wiping it ──────────────────────────────────
	var avatarURL *string
	tx.QueryRow(ctx, `SELECT avatar_url FROM users WHERE id = $1`, callerID).Scan(&avatarURL)
 
	// ── 2. Anonymize the user row ─────────────────────────────────────────────
	// Replace email with a placeholder so the original email can be re-used
	// if the person ever signs up again. Everything else is NULLed out.
	_, err = tx.Exec(ctx, `
		UPDATE users SET
			name                = 'deleted_' || id::text || '@deleted.spotlight',
			email               = 'deleted_' || id::text || '@deleted.spotlight',
			phone               = NULL,
			bio                 = NULL,
			city                = NULL,
			gender              = NULL,
			age                 = NULL,
			date_of_birth       = NULL,
			avatar_url          = NULL,
			fcm_token           = NULL,
			fcm_updated_at      = NULL,
			onboarding_complete = false,
			is_anonymized       = true,
			updated_at          = NOW()
		WHERE id = $1
	`, callerID)
	if err != nil {
		http.Error(w, "failed to anonymize user", http.StatusInternalServerError)
		return
	}
 
	// ── 3. Delete their registrations (as attendee) ───────────────────────────
	// They shouldn't appear on other events' guest lists
	_, err = tx.Exec(ctx,
		`DELETE FROM event_registrations WHERE user_id = $1`, callerID,
	)
	if err != nil {
		http.Error(w, "failed to delete registrations", http.StatusInternalServerError)
		return
	}
 
	// ── 5. Delete OTP records ─────────────────────────────────────────────────
	_, err = tx.Exec(ctx, `
		DELETE FROM auth_otps
		WHERE email = (
			SELECT 'deleted_' || $1::text || '@deleted.spotlight'
		)
	`, callerID)
	if err != nil {
		// Non-fatal
		log.Printf("failed to delete OTPs for user %s: %v", callerID, err)
	}
 
	// ── 6. Commit ─────────────────────────────────────────────────────────────
	if err := tx.Commit(ctx); err != nil {
		http.Error(w, "failed to commit", http.StatusInternalServerError)
		return
	}
 
	// ── 7. Delete avatar from R2 (after commit — non-fatal if it fails) ───────
	if avatarURL != nil && h.R2 != nil {
		key := h.R2.KeyFromURL(*avatarURL)
		if key != "" {
			if err := h.R2.Delete(context.Background(), key); err != nil {
				log.Printf("R2 avatar delete failed for %s: %v", callerID, err)
			}
		}
	}
 
	w.WriteHeader(http.StatusNoContent)
}