package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	clerk "github.com/clerk/clerk-sdk-go/v2"
	gorilllaHandlers "github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"outDrinkMeAPI/handlers"
	"outDrinkMeAPI/middleware"
	"outDrinkMeAPI/services"
)

var (
	dbPool      *pgxpool.Pool
	userService *services.UserService
	docService  *services.DocService
)

func init() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	// Initialize Clerk
	clerkSecretKey := os.Getenv("CLERK_SECRET_KEY")
	if clerkSecretKey == "" {
		log.Fatal("CLERK_SECRET_KEY environment variable is not set")
	}
	clerk.SetKey(clerkSecretKey)
	log.Println("Clerk initialized successfully")

	// Load database URL
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is not set")
	}
	log.Printf("DATABASE_URL loaded: %s", dbURL[:50]+"...")

	// Initialize database connection pool
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	poolConfig, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		log.Fatal("Failed to parse database URL:", err)
	}

	// Configure connection pool
	poolConfig.MaxConns = 25
	poolConfig.MinConns = 5
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute
	poolConfig.HealthCheckPeriod = time.Minute

	dbPool, err = pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		log.Fatal("Failed to create connection pool:", err)
	}

	// Test connection
	if err := dbPool.Ping(ctx); err != nil {
		log.Fatal("Failed to ping database:", err)
	}

	log.Println("Successfully connected to NeonDB")

	// Initialize services
	userService = services.NewUserService(dbPool)

	// Initialize database schema
	// if err := userService.InitSchema(ctx); err != nil {
	// 	log.Fatal("Failed to initialize schema:", err)
	// }
	// log.Println("Database schema initialized")
}

func main() {
	defer func() {
		log.Println("Closing database connection pool...")
		dbPool.Close()
	}()

	// Initialize handlers
	userHandler := handlers.NewUserHandler(userService)
	docHandler := handlers.NewDocHandler(docService)
	webhookHandler := handlers.NewWebhookHandler(userService)

	r := mux.NewRouter()

	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := dbPool.Ping(ctx); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status": "unhealthy", "error": "database connection failed"}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "healthy", "service": "outDrinkMe-api"}`))
	}).Methods("GET")

	r.HandleFunc("/webhooks/clerk", webhookHandler.HandleClerkWebhook).Methods("POST")

	api := r.PathPrefix("/api/v1").Subrouter()

	api.HandleFunc("/privacy-policy", docHandler.ServePrivacyPolicy).Methods("GET")
	api.HandleFunc("/terms-of-services", docHandler.ServeTermsOfServices).Methods("GET")


	// Protected API routes (auth required)
	protected := api.PathPrefix("").Subrouter()
	protected.Use(middleware.ClerkAuthMiddleware)

	protected.HandleFunc("/user", userHandler.GetProfile).Methods("GET")
	protected.HandleFunc("/user/friend-discovery/display-profile", userHandler.FriendDiscoveryDisplayProfile).Methods("GET")
	protected.HandleFunc("/user/update-profile", userHandler.UpdateProfile).Methods("PUT")
	protected.HandleFunc("/user/delete-account", userHandler.DeleteAccount).Methods("DELETE")
	protected.HandleFunc("/user/leaderboard/friends", userHandler.GetFriendsLeaderboard).Methods("GET")
	protected.HandleFunc("/user/leaderboard/global", userHandler.GetGlobalLeaderboard).Methods("GET")
	protected.HandleFunc("/user/friends", userHandler.GetFriends).Methods("GET")
	protected.HandleFunc("/user/friends", userHandler.AddFriend).Methods("POST")
	protected.HandleFunc("/user/friends", userHandler.RemoveFriend).Methods("DELETE")
	protected.HandleFunc("/user/discovery", userHandler.GetDiscovery).Methods("GET")
	protected.HandleFunc("/user/achievements", userHandler.GetAchievements).Methods("GET")
	protected.HandleFunc("/user/drink", userHandler.AddDrinking).Methods("POST")
	protected.HandleFunc("/user/stats", userHandler.GetUserStats).Methods("GET")
	protected.HandleFunc("/user/stats/weekly", userHandler.GetWeeklyDaysDrank).Methods("GET")
	protected.HandleFunc("/user/stats/monthly", userHandler.GetMonthlyDaysDrank).Methods("GET")
	protected.HandleFunc("/user/stats/yearly", userHandler.GetYearlyDaysDrank).Methods("GET")
	protected.HandleFunc("/user/stats/all-time", userHandler.GetAllTimeDaysDrank).Methods("GET")
	protected.HandleFunc("/user/calendar", userHandler.GetCalendar).Methods("GET")
	protected.HandleFunc("/user/search", userHandler.SearchUsers).Methods("GET")


	// CORS configuration
	corsHandler := gorilllaHandlers.CORS(
		gorilllaHandlers.AllowedOrigins([]string{"*"}), // Configure for production
		gorilllaHandlers.AllowedMethods([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
		gorilllaHandlers.AllowedHeaders([]string{"Content-Type", "Authorization"}),
		gorilllaHandlers.ExposedHeaders([]string{"Content-Length"}),
		gorilllaHandlers.AllowCredentials(),
	)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3333"
	}
	port = ":" + port

	server := http.Server{
		Addr:         port,
		Handler:      corsHandler(r),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("Starting server on port %s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Error starting server:", err)
		}
	}()

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	sig := <-sigChan
	log.Println("Got signal:", sig)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Server shutdown complete")
}
