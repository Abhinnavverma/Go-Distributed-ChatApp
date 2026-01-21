# High-Scale Distributed Chat System 🚀

A real-time, horizontally scalable chat application built with Go, Redis, and Docker.

## 🏗 Architecture
- **Backend:** Golang (Gorilla WebSockets)
- **Concurrency:** Goroutines & Channels for non-blocking I/O
- **Scaling:** Redis Pub/Sub for cross-server message synchronization
- **Load Balancing:** Nginx (Round-Robin)
- **Infrastructure:** Docker Compose

## ⚡ Features
- Real-time bidirectional communication
- Heartbeat (Ping/Pong) for connection health
- Auto-reconnection logic
- Horizontally scalable to N instances

## 🚀 How to Run
```bash
docker-compose up --build --scale app=3