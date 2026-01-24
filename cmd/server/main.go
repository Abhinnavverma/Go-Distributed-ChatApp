package main

import (
	"context"
	"flag"
	"go-chat/internal/chat"
	"go-chat/internal/db"
	myMiddleware "go-chat/internal/middleware"
	"go-chat/internal/user"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/redis/go-redis/v9"
)

func main() {
	// 1. Config & Flags
	addr := flag.String("addr", ":8080", "http service address")
	flag.Parse()

	// Get Secrets from Environment (Docker)
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		log.Fatal("❌ DB_DSN is not set")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("❌ JWT_SECRET is not set")
	}

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	// 2. Connect to Database (Platform Layer)
	database, err := db.NewDatabase(dsn)
	if err != nil {
		log.Fatalf("❌ Failed to connect to DB: %v", err)
	}
	log.Println("✅ Connected to PostgreSQL")

	if err := database.AutoMigrate(); err != nil {
		log.Fatalf("❌ Migration failed: %v", err)
	}
	log.Println("✅ Database Schema Initialized")

	// 3. Connect to Redis (Platform Layer)
	redisClient := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	if _, err := redisClient.Ping(context.Background()).Result(); err != nil {
		log.Fatalf("❌ Failed to connect to Redis: %v", err)
	}
	log.Println("✅ Connected to Redis")

	// 4. Initialize User Feature
	// User Repo still uses the raw SQL connection (assuming you didn't change user/repository.go)
	userRepo := user.NewRepository(database.Conn)
	userService := user.NewService(userRepo, jwtSecret)
	userHandler := user.NewHandler(userService)

	// 5. Initialize Chat Feature
	// 🟢 UDPATE 1: ChatRepo now takes the *db.Database wrapper (to access Conn)
	chatRepo := chat.NewRepository(database.Conn)

	// Hub needs Redis + Repo (to fetch participants)
	hub := chat.NewHub(redisClient, chatRepo)

	// Start the Hub Engines
	go hub.Run()
	go hub.SubscribeToRedis()

	// 🟢 UPDATE 2: ChatHandler now needs Repo (for API) + Hub (for WS)
	chatHandler := chat.NewHandler(hub, chatRepo)

	authMiddleware := myMiddleware.NewAuthMiddleware(userService)

	// 6. Define Routes
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Public Routes
	r.Post("/register", userHandler.Register)
	r.Post("/login", userHandler.Login)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	// Protected Routes (Require JWT)
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware.Handle)
		r.Get("/api/users/search", userHandler.SearchUsers)

		// WebSocket (Real-time)
		r.Get("/ws", chatHandler.ServeWs)

		// 🟢 UPDATE 3: New REST API Routes for "WhatsApp" flow
		r.Post("/api/conversations", chatHandler.StartConversation) // Find/Create Chat
		r.Get("/api/messages", chatHandler.GetChatHistory)          // Load History
	})

	log.Printf("🚀 Server starting on %s", *addr)
	if err := http.ListenAndServe(*addr, r); err != nil {
		log.Fatal(err)
	}
}
