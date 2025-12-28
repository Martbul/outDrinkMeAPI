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
	sideQuestService    *services.SideQuestService
	notificationService *services.NotificationService
	fcmService          *notification.FCMService
	photoDumpService    *services.FuncService
	gameManager         *services.DrinnkingGameManager
)

// init() handles Environment, Clerk, and Database Config
func init() {
	// 1. Load Env
	if err := godotenv.Load(); err != nil {
		log.Println("Note: .env file not found, using system env")
	}

	// 2. Clerk
	clerkSecretKey := os.Getenv("CLERK_SECRET_KEY")
	if clerkSecretKey == "" {
		log.Fatal("CLERK_SECRET_KEY environment variable is not set")
	}
	clerk.SetKey(clerkSecretKey)
	log.Println("Clerk initialized successfully")

	// 3. Database Config
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is not set")
	}

	// Create a short context for config parsing (not for connection)
	_, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	poolConfig, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		log.Fatal("Failed to parse database URL:", err)
	}

	// --- OPTIMIZED POOL SETTINGS (For Direct Neon Connection) ---

	// Keep connections alive for 4 minutes.
	// Neon suspends compute after 5 minutes of inactivity.
	// This keeps the connection "hot" while users are active.
	poolConfig.MaxConnIdleTime = 4 * time.Minute

	// Keep at least 1 connection open to avoid TCP handshake lag on every request.
	poolConfig.MinConns = 1

	// Cap max connections to 15.
	// Neon's smallest compute allows ~112 connections.
	// Since we are NOT using the pooler, we must limit this locally.
	poolConfig.MaxConns = 15

	// Jitter the lifetime to prevent mass reconnections.
	poolConfig.MaxConnLifetime = 30 * time.Minute

	// Check health occasionally.
	poolConfig.HealthCheckPeriod = 1 * time.Minute

	// Create the Pool (Lazy connection - does not block startup)
	dbPool, err = pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		log.Fatal("Failed to create connection pool:", err)
	}

	log.Println("Database pool configured (Direct connection)")

	// 4. Initialize Services (Except FCM, which loads in main)
	notificationService = services.NewNotificationService(dbPool)
	userService = services.NewUserService(dbPool, notificationService)
	storeService = services.NewStoreService(dbPool)
	sideQuestService = services.NewSideQuestService(dbPool, notificationService)
	photoDumpService = services.NewFuncService(dbPool)
	gameManager = services.NewDrinnkingGameManager()

	middleware.InitPrometheus()
}

func main() {
	defer func() {
		log.Println("Closing database connection pool...")
		dbPool.Close()
	}()

	// 1. Initialize FCM in Background (Prevents blocking startup)
	go func() {
		fcm, err := notification.NewFCMService("./serviceAccountKey.json")
		if err != nil {
			log.Printf("Warning: Could not initialize FCM: %v", err)
			return
		}
		notificationService.SetPushProvider(fcm)
		log.Println("FCM Push Provider initialized in background")
	}()

	// 2. Setup Handlers
	userHandler := handlers.NewUserHandler(userService)
	docHandler := handlers.NewDocHandler(docService)
	storeHandler := handlers.NewStoreHandler(storeService)
	sideQuestHandler := handlers.NewSideQuestHandler(sideQuestService)
	notificationHandler := handlers.NewNotificationHandler(notificationService)
	webhookHandler := handlers.NewWebhookHandler(userService)
	funcHandler := handlers.NewFuncHandler(photoDumpService)
	drinkingGameHandler := handlers.NewDrinkingGamesHandler(gameManager, userService)

	r := mux.NewRouter()

	r.HandleFunc("/api/v1/drinking-games/ws/{sessionID}", drinkingGameHandler.JoinDrinkingGame)

	standardRouter := r.PathPrefix("/").Subrouter()

	go middleware.CleanupVisitors()

	standardRouter.Use(middleware.RateLimitMiddleware)
	standardRouter.Use(middleware.MonitorMiddleware)

	standardRouter.Handle("/metrics", middleware.BasicAuthMiddleware(promhttp.Handler()))
	standardRouter.PathPrefix("/debug/pprof/").Handler(middleware.PprofSecurityMiddleware(http.DefaultServeMux))

	assetsDir := "./assets"
	fs := http.FileServer(http.Dir(assetsDir))
	standardRouter.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", fs))
	log.Printf("Serving static files from %s at /assets/", assetsDir)

	standardRouter.HandleFunc("/app-ads.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("google.com, pub-1167503921437683, DIRECT, f08c47fec0942fa0"))
	})

	standardRouter.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "healthy", "service": "outDrinkMe-api"}`))
	}).Methods("GET")

	standardRouter.HandleFunc("/webhooks/clerk", webhookHandler.HandleClerkWebhook).Methods("POST")

	api := standardRouter.PathPrefix("/api/v1").Subrouter()

	api.HandleFunc("/drinking-games/public", drinkingGameHandler.GetPublicDrinkingGames).Methods("GET")

	api.HandleFunc("/privacy-policy", docHandler.ServePrivacyPolicy).Methods("GET")
	api.HandleFunc("/terms-of-services", docHandler.ServeTermsOfServices).Methods("GET")
	api.HandleFunc("/delete-account-webpage", userHandler.DeleteAccountPage).Methods("GET")
	api.HandleFunc("/delete-account-details-webpage", userHandler.UpdateAccountPage).Methods("GET")

	protected := api.PathPrefix("").Subrouter()
	protected.Use(middleware.ClerkAuthMiddleware)

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

	protected.HandleFunc("/store", storeHandler.GetStore).Methods("GET")
	protected.HandleFunc("/store/purchase/item", storeHandler.PurchaseStoreItem).Methods("POST")

	protected.HandleFunc("/notifications", notificationHandler.GetNotifications).Methods("GET")
	protected.HandleFunc("/notifications/unread-count", notificationHandler.GetUnreadCount).Methods("GET")
	protected.HandleFunc("/notifications/{id}/read", notificationHandler.MarkAsRead).Methods("PUT")
	protected.HandleFunc("/notifications/read-all", notificationHandler.MarkAllAsRead).Methods("PUT")
	protected.HandleFunc("/notifications/{id}", notificationHandler.DeleteNotification).Methods("DELETE")
	protected.HandleFunc("/notifications/preferences", notificationHandler.GetPreferences).Methods("GET")
	protected.HandleFunc("/notifications/preferences", notificationHandler.UpdatePreferences).Methods("PUT")
	protected.HandleFunc("/notifications/register-device", notificationHandler.RegisterDevice).Methods("POST")
	protected.HandleFunc("/notifications/test", notificationHandler.SendTestNotification).Methods("POST")

	protected.HandleFunc("/sidequest/board", sideQuestHandler.GetSideQuestBoard).Methods("GET")
	protected.HandleFunc("/sidequest", sideQuestHandler.PostNewSideQuest).Methods("POST")
	protected.HandleFunc("/func/create", funcHandler.CreateFunction).Methods("GET")
	protected.HandleFunc("/func/active", funcHandler.GetUserActiveSession).Methods("GET")
	protected.HandleFunc("/func/join", funcHandler.JoinViaQrCode).Methods("POST")
	protected.HandleFunc("/func/data/{id}", funcHandler.GetSessionData).Methods("GET")
	protected.HandleFunc("/func/upload", funcHandler.AddImages).Methods("POST")
	protected.HandleFunc("/func/leave", funcHandler.LeaveFunction).Methods("POST")
	protected.HandleFunc("/func/delete", funcHandler.DeleteImages).Methods("DELETE")
	protected.HandleFunc("/drinking-games/create", drinkingGameHandler.CreateDrinkingGame).Methods("POST")

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
		Addr:         port,
		Handler:      corsHandler(r),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// 3. Start Server Immediately (Non-blocking)
	go func() {
		log.Printf("Starting server on port %s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Error starting server:", err)
		}
	}()

	// 4. Background DB Warmer (Parallel)
	// Even with direct connection, this ensures the DB is awake
	// before the first user request if it has been idle for hours.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		log.Println("Background: Pinging NeonDB to wake it up...")
		if err := dbPool.Ping(ctx); err != nil {
			log.Printf("Warning: NeonDB wake-up ping failed (First request might be slow): %v", err)
		} else {
			log.Println("Success: NeonDB is awake and ready")
		}
	}()

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
