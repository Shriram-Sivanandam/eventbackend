package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	firebase "firebase.google.com/go/v4"
	"github.com/Shriram-Sivanandam/eventbackend/internal/db"
	"github.com/Shriram-Sivanandam/eventbackend/internal/http/handlers"
	"github.com/Shriram-Sivanandam/eventbackend/internal/http/middleware"
	"github.com/Shriram-Sivanandam/eventbackend/internal/scheduler"
	"github.com/Shriram-Sivanandam/eventbackend/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
	"google.golang.org/api/option"
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
	
	scheduler.StartReminderJob(dbPool)

	r2, err := storage.NewR2Client()
	if err != nil {
		log.Fatal(err)
	}

	opt := option.WithAuthCredentialsFile(option.ServiceAccount, os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"))
	firebaseApp, err := firebase.NewApp(context.Background(), nil, opt)
    if err != nil {
        log.Fatalf("failed to init firebase: %v", err)
    }

	firebaseAuth, err := firebaseApp.Auth(context.Background())
    if err != nil {
        log.Fatalf("failed to init firebase auth: %v", err)
    }

	authHandler := &handlers.AuthHandler{DB: dbPool, R2: r2}
	eventsHandler := &handlers.EventsHandler{DB: dbPool, R2: r2, FirebaseAuth: firebaseAuth}
	usersHandler := &handlers.UsersHandler{DB: dbPool}
	tagsHandler := &handlers.TagsHandler{DB: dbPool}
	chatHandler  := &handlers.ChatHandler{DB: dbPool, FirebaseAuth: firebaseAuth}

	r := chi.NewRouter()

	r.Post("/auth/request-otp", authHandler.RequestOTP)
	r.Post("/auth/verify-otp", authHandler.VerifyOTP)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	r.Group(func(protected chi.Router) {
		protected.Use(middleware.AuthMiddleware)
		protected.Patch("/auth/me", authHandler.UpdateProfile)
		protected.Post("/auth/fcm-token", authHandler.SaveFCMToken)

		protected.Post("/events", eventsHandler.CreateEvent)

		protected.Get("/events/unrated", eventsHandler.GetUnratedEvents) 
		protected.Post("/events/{id}/rate", eventsHandler.SubmitRating)
		protected.Post("/events/{id}/dismiss-rating-prompt", eventsHandler.DismissRatingPrompt)
		protected.Patch("/events/{id}/registrations/{userID}", eventsHandler.UpdateRegistrationStatus)
		protected.Get("/events/{id}/dashboard", eventsHandler.GetEventDashboard)
		protected.Post("/events/{id}/chat/token", chatHandler.GetChatToken)
		protected.Delete("/events/{id}/leave", eventsHandler.LeaveEvent)
		protected.Get("/events/{id}", eventsHandler.GetEventByID)
		protected.Get("/events", eventsHandler.GetEvents)
		protected.Get("/events/registered", eventsHandler.GetRegisteredEvents)
		
		protected.Post("/events/{id}/join", eventsHandler.JoinEvent)
		protected.Delete("/events/{id}", eventsHandler.CancelEvent)

		protected.Get("/users/{id}/profile", usersHandler.GetHostProfile)

		protected.Get("/tags", tagsHandler.GetTags)

		protected.Get("/auth/me", authHandler.Me)
		protected.Delete("/auth/me", authHandler.DeleteAccount)

		protected.Get("/chats", chatHandler.GetChatList)
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
