package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/Shriram-Sivanandam/eventbackend/internal/db"
	"github.com/Shriram-Sivanandam/eventbackend/internal/http/handlers"
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

	eventsHandler := &handlers.EventsHandler{DB: dbPool}

	r := chi.NewRouter()

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
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

	r.Get("/events", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5 * time.Second)
		defer cancel()

		events, err := db.GetEvents(ctx, dbPool)

		if err != nil {
			http.Error(w, "failed to fetch events", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(events)
	})

	r.Post("/events", eventsHandler.CreateEvent)

	server := &http.Server {
		Addr : ":8080",
		Handler: r,
		ReadTimeout: 5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	log.Println("running")
	log.Println(server.ListenAndServe())
}
