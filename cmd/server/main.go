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
		// Fallback for local testing
		log.Fatal("❌ DB_DSN is not set in environment or .env file")
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
	// Test Redis connection
	if _, err := redisClient.Ping(context.Background()).Result(); err != nil {
		log.Fatalf("❌ Failed to connect to Redis: %v", err)
	}
	log.Println("✅ Connected to Redis")

	// 4. Initialize User Feature (Identity)
	// Notice: We pass database.Conn (the raw *sql.DB) to the repo
	userRepo := user.NewRepository(database.Conn)
	userService := user.NewService(userRepo, jwtSecret)
	userHandler := user.NewHandler(userService)

	// 5. Initialize Chat Feature (Real-time)
	chatRepo := chat.NewRepository(database.Conn)
	hub := chat.NewHub(redisClient, chatRepo)

	// ⚠️ CRITICAL: Start the Hub in a separate goroutine!
	// If you forget this, the chat will never broadcast messages.
	go hub.Run()
	go hub.SubscribeToRedis() // Listen for messages from other containers

	chatHandler := chat.NewHandler(hub, userService)
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

	// Protected Routes
	r.Group(func(r chi.Router) {
		// Apply our custom middleware
		r.Use(authMiddleware.Handle)
		r.Get("/ws", chatHandler.ServeWs)
	})

	log.Printf("🚀 Server starting on %s", *addr)
	if err := http.ListenAndServe(*addr, r); err != nil {
		log.Fatal(err)
	}
}
