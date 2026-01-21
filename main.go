package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/redis/go-redis/v9"
)

var addr = flag.String("addr", ":8080", "http service address")

func main() {
	flag.Parse()
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	redisClient := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	_, ok := redisClient.Ping(context.Background()).Result()
	if ok != nil {
		log.Fatal("❌ Could not connect to Redis: ", ok)
	}
	log.Println("✅ Connected to Redis!")
	// 1. Initialize the Hub
	hub := newHub(redisClient)
	// 2. Run the Hub in a background Goroutine
	go hub.run()

	// 3. Define Routes
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})

	// Serve the frontend (we'll make index.html in a second)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	log.Println("🚀 Server started on :8080")
	err := http.ListenAndServe(*addr, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
