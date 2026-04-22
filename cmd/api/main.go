package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/Shriram-Sivanandam/eventbackend/internal/db"
	"github.com/Shriram-Sivanandam/eventbackend/internal/http/handlers"
	"github.com/Shriram-Sivanandam/eventbackend/internal/http/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	dbPool, err := db.Connect()
	if err != nil {
		log.Fatal(err)
	}
	defer dbPool.Close()

	log.Println("connected")

	authHandler := &handlers.AuthHandler{DB: dbPool}
	eventsHandler := &handlers.EventsHandler{DB: dbPool}
	usersHandler := &handlers.UsersHandler{DB: dbPool}
	tagsHandler := &handlers.TagsHandler{DB: dbPool}

	r := chi.NewRouter()

	r.Post("/auth/request-otp", authHandler.RequestOTP)
	r.Post("/auth/verify-otp", authHandler.VerifyOTP)

	fs := http.FileServer(http.Dir("./uploads"))
	r.Handle("/uploads/*", http.StripPrefix("/uploads/", fs))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	r.Group(func(protected chi.Router) {
		protected.Use(middleware.AuthMiddleware)
		protected.Patch("/auth/me", authHandler.UpdateProfile)

		protected.Post("/events", eventsHandler.CreateEvent)

		protected.Get("/events/unrated", eventsHandler.GetUnratedEvents) 
		protected.Post("/events/{id}/rate", eventsHandler.SubmitRating)
		protected.Post("/events/{id}/dismiss-rating-prompt", eventsHandler.DismissRatingPrompt)
		protected.Patch("/events/{id}/registrations/{userID}", eventsHandler.UpdateRegistrationStatus)
		protected.Get("/events/{id}/dashboard", eventsHandler.GetEventDashboard)
		protected.Get("/events/{id}", eventsHandler.GetEventByID)
		protected.Get("/events", eventsHandler.GetEvents)
		protected.Get("/events/registered", eventsHandler.GetRegisteredEvents)
		
		protected.Post("/events/{id}/join", eventsHandler.JoinEvent)
		protected.Delete("/events/{id}", eventsHandler.CancelEvent)

		protected.Get("/users/{id}/profile", usersHandler.GetHostProfile)

		protected.Get("/tags", tagsHandler.GetTags)

		protected.Get("/auth/me", authHandler.Me)
	})

	r.Get("/users", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5 * time.Second)
		defer cancel()

		users, err := db.GetUsers(ctx, dbPool)

		if err != nil {
			http.Error(w, "failed to fetch users", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(users)
	})

	server := &http.Server {
		Addr : ":8080",
		Handler: r,
		ReadTimeout: 5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	log.Println("running")
	log.Println(server.ListenAndServe())
}
