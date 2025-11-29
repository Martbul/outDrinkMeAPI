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
	photoDumpService    *services.PhotoDumpService
	gameManager         *services.DrinnkingGameManager
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
	notificationService = services.NewNotificationService(dbPool)

	userService = services.NewUserService(dbPool, notificationService)
	storeService = services.NewStoreService(dbPool)
	sideQuestService = services.NewSideQuestService(dbPool, notificationService)
	photoDumpService = services.NewPhotoDumpService(dbPool)
	fcmService, err = notification.NewFCMService("./serviceAccountKey.json")
	gameManager = services.NewDrinnkingGameManager()

	if err != nil {
		log.Printf("WARNING: FCM not initialized: %v", err)
	} else {
		notificationService.SetPushProvider(fcmService)
	}

	middleware.InitPrometheus()
}

func main() {
	defer func() {
		log.Println("Closing database connection pool...")
		dbPool.Close()
	}()

	// Initialize handlers
	userHandler := handlers.NewUserHandler(userService)
	docHandler := handlers.NewDocHandler(docService)
	storeHandler := handlers.NewStoreHandler(storeService)
	sideQuestHandler := handlers.NewSideQuestHandler(sideQuestService)
	notificationHandler := handlers.NewNotificationHandler(notificationService)
	webhookHandler := handlers.NewWebhookHandler(userService)
	photoDumpHandler := handlers.NewPhotoDumpHandler(photoDumpService)
	drinkingGameHandler := handlers.NewDrinkingGamesHandler(gameManager)

	r := mux.NewRouter()

	go middleware.CleanupVisitors()

	r.Use(middleware.RateLimitMiddleware)

	r.Use(middleware.MonitorMiddleware)

	r.Handle("/metrics", middleware.BasicAuthMiddleware(promhttp.Handler()))

	//pprof attaches to http.DefaultServeMux, so we forward traffic there
	r.PathPrefix("/debug/pprof/").Handler(middleware.PprofSecurityMiddleware(http.DefaultServeMux))

	assetsDir := "./assets"
	fs := http.FileServer(http.Dir(assetsDir))
	r.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", fs))
	log.Printf("Serving static files from %s at /assets/", assetsDir)

	r.HandleFunc("/app-ads.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("google.com, pub-1167503921437683, DIRECT, f08c47fec0942fa0"))
	})

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
	protected.HandleFunc("/user/mix-timeline", userHandler.GetMixTimeline).Methods("GET")
	protected.HandleFunc("/user/friends", userHandler.AddFriend).Methods("POST")
	protected.HandleFunc("/user/friends", userHandler.RemoveFriend).Methods("DELETE")
	protected.HandleFunc("/user/discovery", userHandler.GetDiscovery).Methods("GET")
	protected.HandleFunc("/user/achievements", userHandler.GetAchievements).Methods("GET")
	protected.HandleFunc("/user/drink", userHandler.AddDrinking).Methods("POST")
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
	protected.HandleFunc("/user/feedback", userHandler.AddUserFeedback).Methods("POST")

	protected.HandleFunc("/store", storeHandler.GetStore).Methods("GET")
	protected.HandleFunc("/store/purchase/item", storeHandler.PurchaseStoreItem).Methods("POST")
	// protected.HandleFunc("/store/purchase/gems", storeHandler.PurchaseGems).Methods("POST")

	protected.HandleFunc("/notifications", notificationHandler.GetNotifications).Methods("GET")
	protected.HandleFunc("/notifications/unread-count", notificationHandler.GetUnreadCount).Methods("GET")
	protected.HandleFunc("/notifications/:id/read", notificationHandler.MarkAsRead).Methods("PUT")
	protected.HandleFunc("/notifications/read-all", notificationHandler.MarkAllAsRead).Methods("PUT")
	protected.HandleFunc("/notifications/:id", notificationHandler.DeleteNotification).Methods("DELETE")
	protected.HandleFunc("/notifications/preferences", notificationHandler.GetPreferences).Methods("GET")
	protected.HandleFunc("/notifications/preferences", notificationHandler.UpdatePreferences).Methods("PUT")
	protected.HandleFunc("/notifications/register-device", notificationHandler.RegisterDevice).Methods("POST")
	protected.HandleFunc("/notifications/test", notificationHandler.SendTestNotification).Methods("POST")

	protected.HandleFunc("/sidequest/board", sideQuestHandler.GetSideQuestBoard).Methods("GET")
	protected.HandleFunc("/sidequest", sideQuestHandler.PostNewSideQuest).Methods("POST")
	// protected.HandleFunc("sidequest/:id/complete", sideQuestHandler.PostCompletion).Methods("POST") //Up to 3 iamges(sending the pics to the user that has added the quest for aprovam, if aproved -> grand the gems(fromk the quester account) to the completer)
	// protected.HandleFunc("sidequest/:id/complete", sideQuestHandler.PostNewSideQuest).Methods("POST")

	protected.HandleFunc("/photo-dump/generate", photoDumpHandler.GenerateQrCode).Methods("GET")
	protected.HandleFunc("/photo-dump/scan", photoDumpHandler.JoinViaQrCode).Methods("POST")
	protected.HandleFunc("/photo-dump/:sesionId", photoDumpHandler.GetSessionData).Methods("GET")
	protected.HandleFunc("/photo-dump/:sesionId", photoDumpHandler.AddImages).Methods("POST")

	protected.HandleFunc("/drinking-games/create", drinkingGameHandler.CreateDrinkingGame).Methods("POST")


    // --- WEBSOCKET ROUTES (Join the game) ---
    // Note: Use handleFunc, not Handle, to access vars easily
    
    // Using a separate route for WS prevents Middleware conflict if needed, 
    // but here we attach to the main router.
    protected.HandleFunc("/api/v1/games/ws/{sessionID}",drinkingGameHandler.JoinDrinkingGame).Methods("GET") // .Methods("GET") is implied for WS
	 
//  r.HandleFunc("/api/v1/games/ws/{sessionID}", func(w http.ResponseWriter, r *http.Request) {
//         vars := mux.Vars(r)
//         sessionID := vars["sessionID"]

//         // 1. Validate Session Exists
//         session, exists := gameManager.GetSession(sessionID)
//         if !exists {
//             http.Error(w, "Game session not found", http.StatusNotFound)
//             return
//         }

//         // 2. Upgrade Connection
//         conn, err := upgrader.Upgrade(w, r, nil)
//         if err != nil {
//             log.Println(err)
//             return
//         }

//         // 3. Register User to Session
//         client := &game.Client{
//             Session: session,
//             Conn:    conn,
//             Send:    make(chan []byte, 256),
//         }
        
//         // Start Pumps
//         client.Session.Register <- client
//         go client.WritePump()
//         go client.ReadPump()

//     }) // .Methods("GET") is implied for WS
	 
	// CORS configuration
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
