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
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"outDrinkMeAPI/handlers"
	"outDrinkMeAPI/internal/notification"
	"outDrinkMeAPI/middleware"
	"outDrinkMeAPI/services"

	_ "net/http/pprof"
)

var (
	dbPool              *pgxpool.Pool
	userService         *services.UserService
	docService          *services.DocService
	storeService        *services.StoreService
	notificationService *services.NotificationService
	photoDumpService    *services.FuncService
	gameManager         *services.DrinnkingGameManager
)

func main() {
	// 1. Load Environment Variables
	if err := godotenv.Load(); err != nil {
		log.Println("Note: .env file not found, using system env")
	}

	clerkSecretKey := os.Getenv("CLERK_SECRET_KEY")
	if clerkSecretKey == "" {
		log.Fatal("CLERK_SECRET_KEY environment variable is not set")
	}
	clerk.SetKey(clerkSecretKey)
	log.Println("Clerk initialized")

	// Initialize Prometheus (moved from init)
	middleware.InitPrometheus()

	// 2. Configure Database (Non-blocking)
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is not set")
	}

	poolConfig, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		log.Fatal("Failed to parse database URL:", err)
	}

	// --- OPTIMIZED POOL SETTINGS ---
	// 4 minutes idle time (Neon sleeps at 5 mins)
	poolConfig.MaxConnIdleTime = 4 * time.Minute
	// CRITICAL: Set MinConns to 0 so the server starts immediately
	poolConfig.MinConns = 0
	poolConfig.MaxConns = 15
	poolConfig.MaxConnLifetime = 30 * time.Minute
	poolConfig.HealthCheckPeriod = 1 * time.Minute

	// Create the Pool (This is now instant because MinConns=0)
	dbPool, err = pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		log.Fatal("Failed to create connection pool:", err)
	}
	log.Println("Database pool configured (Lazy connection)")

	defer func() {
		log.Println("Closing database connection pool...")
		dbPool.Close()
	}()

	// 3. Initialize Services
	notificationService = services.NewNotificationService(dbPool)
	userService = services.NewUserService(dbPool, notificationService)
	storeService = services.NewStoreService(dbPool)
	photoDumpService = services.NewFuncService(dbPool)
	gameManager = services.NewDrinnkingGameManager()
	docService = services.NewDocService(dbPool)

	// 4. Initialize Handlers
	userHandler := handlers.NewUserHandler(userService)
	docHandler := handlers.NewDocHandler(docService)
	storeHandler := handlers.NewStoreHandler(storeService)
	notificationHandler := handlers.NewNotificationHandler(notificationService)
	webhookHandler := handlers.NewWebhookHandler(userService)
	funcHandler := handlers.NewFuncHandler(photoDumpService)
	drinkingGameHandler := handlers.NewDrinkingGamesHandler(gameManager, userService)

	// 5. Initialize Background Tasks
	go func() {
		fcm, err := notification.NewFCMService("./serviceAccountKey.json")
		if err != nil {
			log.Printf("Warning: Could not initialize FCM: %v", err)
			return
		}
		notificationService.SetPushProvider(fcm)
		log.Println("FCM Push Provider initialized in background")
	}()

	go middleware.CleanupVisitors()

	// --- OPTIMIZED DB WARMER ---
	// Runs in parallel. Tries to wake up Neon 3 times.
	go func() {
		for i := 0; i < 3; i++ {
			// Small delay to let network stack settle
			time.Sleep(500 * time.Millisecond)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			log.Printf("Background: Pinging NeonDB to wake it up (Attempt %d/3)...", i+1)
			
			if err := dbPool.Ping(ctx); err == nil {
				log.Println("Success: NeonDB is awake and ready")
				cancel()
				return
			} else {
				log.Printf("Ping failed: %v", err)
			}
			cancel()
		}
		log.Println("Warning: NeonDB warm-up failed after retries. First request might be slow.")
	}()

	// 6. Router Setup
	r := mux.NewRouter()

	// --- CRITICAL OPTIMIZATION: Health Check ---
	// Defined on the Root Router 'r' so it BYPASSES middleware.
	// This ensures UptimeRobot gets a 200 OK instantly, even if the DB is cold.
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "healthy", "service": "outDrinkMe-api"}`))
	}).Methods("GET")

	// Websocket Route (needs to be on root usually or handle upgrades carefully)
	r.HandleFunc("/api/v1/drinking-games/ws/{sessionID}", drinkingGameHandler.JoinDrinkingGame)

	// Subrouter for standard API traffic (Attaching Middleware here)
	standardRouter := r.PathPrefix("/").Subrouter()
	standardRouter.Use(middleware.RateLimitMiddleware)
	standardRouter.Use(middleware.MonitorMiddleware)

	// Observability Routes
	standardRouter.Handle("/metrics", middleware.BasicAuthMiddleware(promhttp.Handler()))
	standardRouter.PathPrefix("/debug/pprof/").Handler(middleware.PprofSecurityMiddleware(http.DefaultServeMux))

	// Static Assets
	assetsDir := "./assets"
	fs := http.FileServer(http.Dir(assetsDir))
	standardRouter.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", fs))
	log.Printf("Serving static files from %s at /assets/", assetsDir)

	// Ads / Metadata
	standardRouter.HandleFunc("/app-ads.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("google.com, pub-1167503921437683, DIRECT, f08c47fec0942fa0"))
	})

	// Webhooks
	standardRouter.HandleFunc("/webhooks/clerk", webhookHandler.HandleClerkWebhook).Methods("POST")

	// --- API v1 Routes ---
	api := standardRouter.PathPrefix("/api/v1").Subrouter()

	// Public Routes
	api.HandleFunc("/drinking-games/public", drinkingGameHandler.GetPublicDrinkingGames).Methods("GET")
	api.HandleFunc("/privacy-policy", docHandler.ServePrivacyPolicy).Methods("GET")
	api.HandleFunc("/terms-of-services", docHandler.ServeTermsOfServices).Methods("GET")
	api.HandleFunc("/delete-account-webpage", userHandler.DeleteAccountPage).Methods("GET")
	api.HandleFunc("/delete-account-details-webpage", userHandler.UpdateAccountPage).Methods("GET")

	// Protected Routes (Require Clerk Auth)
	protected := api.PathPrefix("").Subrouter()
	protected.Use(middleware.ClerkAuthMiddleware)

	// User Routes
	protected.HandleFunc("/user", userHandler.GetProfile).Methods("GET")
	protected.HandleFunc("/user/friend-discovery/display-profile", userHandler.FriendDiscoveryDisplayProfile).Methods("GET")
	protected.HandleFunc("/user/update-profile", userHandler.UpdateProfile).Methods("PUT")
	protected.HandleFunc("/user/delete-account", userHandler.DeleteAccount).Methods("DELETE")
	protected.HandleFunc("/user/leaderboards", userHandler.GetLeaderboards).Methods("GET")
	protected.HandleFunc("/user/friends", userHandler.GetFriends).Methods("GET")
	protected.HandleFunc("/user/your-mix", userHandler.GetYourMix).Methods("GET")
	protected.HandleFunc("/user/global-mix", userHandler.GetGlobalMix).Methods("GET")
	protected.HandleFunc("/user/map-friend-posts", userHandler.GetUserFriendsPosts).Methods("GET")
	protected.HandleFunc("/user/mix-timeline", userHandler.GetMixTimeline).Methods("GET")
	protected.HandleFunc("/user/friends", userHandler.AddFriend).Methods("POST")
	protected.HandleFunc("/user/friends", userHandler.RemoveFriend).Methods("DELETE")
	protected.HandleFunc("/user/discovery", userHandler.GetDiscovery).Methods("GET")
	protected.HandleFunc("/user/achievements", userHandler.GetAchievements).Methods("GET")
	protected.HandleFunc("/user/drink", userHandler.AddDrinking).Methods("POST")
	protected.HandleFunc("/user/memory-wall/{postId}", userHandler.GetMemoryWall).Methods("GET")
	protected.HandleFunc("/user/memory-wall", userHandler.AddMemoryToWall).Methods("POST")
	protected.HandleFunc("/user/drink", userHandler.RemoveDrinking).Methods("DELETE")
	protected.HandleFunc("/user/drunk-thought", userHandler.GetDrunkThought).Methods("GET")
	protected.HandleFunc("/user/drunk-thought", userHandler.AddDrunkThought).Methods("POST")
	protected.HandleFunc("/user/stats", userHandler.GetUserStats).Methods("GET")
	protected.HandleFunc("/user/stats/weekly", userHandler.GetWeeklyDaysDrank).Methods("GET")
	protected.HandleFunc("/user/stats/monthly", userHandler.GetMonthlyDaysDrank).Methods("GET")
	protected.HandleFunc("/user/stats/yearly", userHandler.GetYearlyDaysDrank).Methods("GET")
	protected.HandleFunc("/user/stats/all-time", userHandler.GetAllTimeDaysDrank).Methods("GET")
	protected.HandleFunc("/user/calendar", userHandler.GetCalendar).Methods("GET")
	protected.HandleFunc("/user/search", userHandler.SearchUsers).Methods("GET")
	protected.HandleFunc("/user/search-db-alcohol", userHandler.SearchDbAlcohol).Methods("GET")
	protected.HandleFunc("/user/alcohol-collection", userHandler.GetUserAlcoholCollection).Methods("GET")
	protected.HandleFunc("/user/alcohol-collection", userHandler.RemoveAlcoholCollectionItem).Methods("DELETE")
	protected.HandleFunc("/user/mix-videos", userHandler.GetMixVideoFeed).Methods("GET")
	protected.HandleFunc("/user/mix-videos", userHandler.AddMixVideo).Methods("POST")
	protected.HandleFunc("/user/mix-video-chips", userHandler.AddChipsToVideo).Methods("POST")
	protected.HandleFunc("/user/drunk-friend-thoughts", userHandler.GetDrunkFriendThoughts).Methods("GET")
	protected.HandleFunc("/user/inventory", userHandler.GetUserInventory).Methods("GET")
	protected.HandleFunc("/user/alcoholisum_chart", userHandler.GetAlcoholismChart).Methods("GET")
	protected.HandleFunc("/user/feedback", userHandler.AddUserFeedback).Methods("POST")
	protected.HandleFunc("/user/stories", userHandler.GetStories).Methods("GET")
	protected.HandleFunc("/user/stories", userHandler.AddStory).Methods("POST")
	protected.HandleFunc("/user/stories/{story_id}", userHandler.DeleteStory).Methods("DELETE")
	protected.HandleFunc("/user/stories/relate", userHandler.RelateStory).Methods("POST")
	protected.HandleFunc("/user/stories/seen", userHandler.MarkStoryAsSeen).Methods("POST")
	protected.HandleFunc("/user/user-stories", userHandler.GetAllUserStories).Methods("GET")
	protected.HandleFunc("/min-version", docHandler.GetAppMinVersion).Methods("GET")

	// Store Routes
	protected.HandleFunc("/store", storeHandler.GetStore).Methods("GET")
	protected.HandleFunc("/store/purchase/item", storeHandler.PurchaseStoreItem).Methods("POST")

	// Notification Routes
	protected.HandleFunc("/notifications", notificationHandler.GetNotifications).Methods("GET")
	protected.HandleFunc("/notifications/unread-count", notificationHandler.GetUnreadCount).Methods("GET")
	protected.HandleFunc("/notifications/{id}/read", notificationHandler.MarkAsRead).Methods("PUT")
	protected.HandleFunc("/notifications/read-all", notificationHandler.MarkAllAsRead).Methods("PUT")
	protected.HandleFunc("/notifications/{id}", notificationHandler.DeleteNotification).Methods("DELETE")
	protected.HandleFunc("/notifications/preferences", notificationHandler.GetPreferences).Methods("GET")
	protected.HandleFunc("/notifications/preferences", notificationHandler.UpdatePreferences).Methods("PUT")
	protected.HandleFunc("/notifications/register-device", notificationHandler.RegisterDevice).Methods("POST")
	protected.HandleFunc("/notifications/test", notificationHandler.SendTestNotification).Methods("POST")

	// Func (Photo Dump) Routes
	protected.HandleFunc("/func/create", funcHandler.CreateFunction).Methods("GET")
	protected.HandleFunc("/func/active", funcHandler.GetUserActiveSession).Methods("GET")
	protected.HandleFunc("/func/join", funcHandler.JoinViaQrCode).Methods("POST")
	protected.HandleFunc("/func/data/{id}", funcHandler.GetSessionData).Methods("GET")
	protected.HandleFunc("/func/upload", funcHandler.AddImages).Methods("POST")
	protected.HandleFunc("/func/leave", funcHandler.LeaveFunction).Methods("POST")
	protected.HandleFunc("/func/delete", funcHandler.DeleteImages).Methods("DELETE")

	// Drinking Game Routes
	protected.HandleFunc("/drinking-games/create", drinkingGameHandler.CreateDrinkingGame).Methods("POST")

	// 7. CORS Configuration
	corsHandler := gorilllaHandlers.CORS(
		gorilllaHandlers.AllowedOrigins([]string{"*"}),
		gorilllaHandlers.AllowedMethods([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
		gorilllaHandlers.AllowedHeaders([]string{"Content-Type", "Authorization", "X-Pprof-Secret"}),
		gorilllaHandlers.ExposedHeaders([]string{"Content-Length"}),
		gorilllaHandlers.AllowCredentials(),
	)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3333"
	}
	port = ":" + port

	server := http.Server{
		Addr:        port,
		Handler:     corsHandler(r),
		ReadTimeout: 10 * time.Second,
		
		// --- CRITICAL OPTIMIZATION ---
		// Increased to 60s. This prevents the "Connection Reset" error 
		// if the DB takes 30s to wake up.
		WriteTimeout: 60 * time.Second,
		
		IdleTimeout:  120 * time.Second,
	}

	// 8. Start Server (Non-blocking)
	go func() {
		log.Printf("Starting server on port %s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Error starting server:", err)
		}
	}()

	// 9. Graceful Shutdown
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