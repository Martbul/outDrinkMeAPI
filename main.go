package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/PaddleHQ/paddle-go-sdk"
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
	venueService        *services.VenueService
	paddleService       *services.PaddleService
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Note: .env file not found, using system env")
	}

	clerkSecretKey := os.Getenv("CLERK_SECRET_KEY")
	if clerkSecretKey == "" {
		log.Fatal("CLERK_SECRET_KEY environment variable is not set")
	}
	clerk.SetKey(clerkSecretKey)
	log.Println("Clerk initialized")

	middleware.InitPrometheus()

	paddleClient, err := paddle.NewSandbox(
		os.Getenv("PADDLE_API_KEY"),
	)
	if err != nil {
		log.Fatalf("Failed to init Paddle client: %v", err)
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is not set")
	}

	poolConfig, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		log.Fatal("Failed to parse database URL:", err)
	}

	poolConfig.MaxConnIdleTime = 30 * time.Second
	poolConfig.MinConns = 0
	poolConfig.MaxConns = 15
	poolConfig.MaxConnLifetime = 5 * time.Minute
	poolConfig.HealthCheckPeriod = 24 * time.Hour

	dbPool, err = pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		log.Fatal("Failed to create connection pool:", err)
	}
	log.Println("Database pool configured (Lazy connection)")

	defer func() {
		log.Println("Closing database connection pool...")
		dbPool.Close()
	}()

	notificationService = services.NewNotificationService(dbPool)
	userService = services.NewUserService(dbPool, notificationService)
	storeService = services.NewStoreService(dbPool)
	photoDumpService = services.NewFuncService(dbPool)
	gameManager = services.NewDrinnkingGameManager()
	docService = services.NewDocService(dbPool)
	venueService = services.NewVenueService(dbPool)
	paddleService = services.NewPaddleService(paddleClient)

	userHandler := handlers.NewUserHandler(userService)
	docHandler := handlers.NewDocHandler(docService)
	storeHandler := handlers.NewStoreHandler(storeService)
	notificationHandler := handlers.NewNotificationHandler(notificationService)
	webhookHandler := handlers.NewWebhookHandler(userService)
	funcHandler := handlers.NewFuncHandler(photoDumpService)
	drinkingGameHandler := handlers.NewDrinkingGamesHandler(gameManager, userService)
	venueHandler := handlers.NewVenueHandler(venueService)
	paddleHandler := handlers.NewPaddleHandler(paddleService)

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

	go func() {
		for i := 0; i < 3; i++ {
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

	r := mux.NewRouter()

	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "healthy", "service": "outDrinkMe-api"}`))
	}).Methods("GET")

	r.HandleFunc("/api/v1/drinking-games/ws/{sessionID}", drinkingGameHandler.JoinDrinkingGame)

	standardRouter := r.PathPrefix("/").Subrouter()
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
	standardRouter.HandleFunc("/", docHandler.ServeHome).Methods("GET")

	standardRouter.HandleFunc("/webhooks/clerk", webhookHandler.HandleClerkWebhook).Methods("POST")
	standardRouter.HandleFunc("/webhooks/stripe", webhookHandler.HandleStripeWebhook).Methods("POST")
	standardRouter.HandleFunc("/webhooks/paddle", paddleHandler.PaddleWebhookHandler).Methods("POST")
	standardRouter.HandleFunc("/payment/success", paddleHandler.HandlePaymentSuccess).Methods("GET")

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
	protected.HandleFunc("/user/subscription", userHandler.GetSubscriptionDetails).Methods("GET")
	protected.HandleFunc("/user/subscription", userHandler.Subscribe).Methods("POST")

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

	protected.HandleFunc("/func/create", funcHandler.CreateFunction).Methods("GET")
	protected.HandleFunc("/func/active", funcHandler.GetUserActiveSession).Methods("GET")
	protected.HandleFunc("/func/join", funcHandler.JoinViaQrCode).Methods("POST")
	protected.HandleFunc("/func/data/{id}", funcHandler.GetSessionData).Methods("GET")
	protected.HandleFunc("/func/upload", funcHandler.AddImages).Methods("POST")
	protected.HandleFunc("/func/leave", funcHandler.LeaveFunction).Methods("POST")
	protected.HandleFunc("/func/delete", funcHandler.DeleteImages).Methods("DELETE")
	protected.HandleFunc("/drinking-games/create", drinkingGameHandler.CreateDrinkingGame).Methods("POST")

	protected.HandleFunc("/venues", venueHandler.GetAllVenues).Methods("GET")
	protected.HandleFunc("/venues/employee", venueHandler.GetEmployeeDetails).Methods("GET")
	protected.HandleFunc("/venues/employee", venueHandler.AddEmployeeToVenue).Methods("POST")
	protected.HandleFunc("/venues/employee", venueHandler.RemoveEmployeeFromVenue).Methods("DELETE")
	protected.HandleFunc("/venues/scan", venueHandler.AddScanData).Methods("POST")

	protected.HandleFunc("/paddle/price", paddleHandler.GetPrices).Methods("GET")
	protected.HandleFunc("/paddle/transaction", paddleHandler.CreateTransaction).Methods("POST")

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
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("Starting server on port %s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Error starting server:", err)
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
