package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	BaseURL   = "http://localhost"
	WSURL     = "ws://localhost/ws"
	UserCount = 500 // ⚠️ Start small (50 pairs = 100 users). Database might choke on 1000 immediately.
	MsgCount  = 20  // Messages per user
)

type AuthResponse struct {
	Token    string `json:"access_token"`
	Username string `json:"username"`
}

type ConversationResponse struct {
	ID int `json:"conversation_id"`
}

func main() {
	log.Printf("🔥 STARTING STRESS TEST: %d Users, %d Messages each...", UserCount*2, MsgCount)
	var wg sync.WaitGroup

	// We will create pairs: User 0 talks to User 1, User 2 talks to User 3...
	for i := 0; i < UserCount; i++ {
		wg.Add(1)
		go func(pairID int) {
			defer wg.Done()
			runPair(pairID)
		}(i)
	}

	wg.Wait()
	log.Println("✅ LOAD TEST COMPLETE")
}

func runPair(pairID int) {
	// 1. Define Users (e.g., user_0_a, user_0_b)
	userA := fmt.Sprintf("u_%d_a", pairID)
	userB := fmt.Sprintf("u_%d_b", pairID)
	pass := "password123"

	// 2. Register & Login
	tokenA, _ := authenticate(userA, pass)
	tokenB, idB := authenticate(userB, pass)

	if tokenA == "" || tokenB == "" {
		return // Failed auth
	}

	// 3. User A starts conversation with User B
	convID := createConversation(tokenA, idB)
	if convID == 0 {
		return
	}

	// 4. Start WebSocket Spam (Both sides)
	var wsWg sync.WaitGroup
	wsWg.Add(2)

	go spamChat(&wsWg, tokenA, convID, userA)
	go spamChat(&wsWg, tokenB, convID, userB)

	wsWg.Wait()
}

// authenticate registers (ignores error if exists) and logs in
func authenticate(username, password string) (string, int) {
	// Register (Ignore error, might already exist)
	postJSON("/register", map[string]string{"username": username, "password": password})

	// Login
	resp, err := postJSON("/login", map[string]string{"username": username, "password": password})
	if err != nil {
		log.Printf("❌ Login Failed [%s]: %v", username, err)
		return "", 0
	}

	var data AuthResponse
	json.NewDecoder(resp.Body).Decode(&data)
	resp.Body.Close()

	// We need the ID. Hack: Search for self to get ID
	// Real app would return ID in login, but we can just use search
	id := searchUserID(data.Token, username)
	return data.Token, id
}

// searchUserID finds the user ID by username
func searchUserID(token, username string) int {
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/users/search?q=%s", BaseURL, username), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()

	var users []struct {
		ID       int    `json:"id"`
		Username string `json:"username"`
	}
	json.NewDecoder(resp.Body).Decode(&users)

	for _, u := range users {
		if u.Username == username {
			return u.ID
		}
	}
	return 0
}

func createConversation(token string, targetID int) int {
	body := map[string]int{"target_id": targetID}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", BaseURL+"/api/conversations", bytes.NewBuffer(jsonBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		log.Printf("❌ Create Chat Failed: %v", err)
		return 0
	}
	defer resp.Body.Close()

	var data ConversationResponse
	json.NewDecoder(resp.Body).Decode(&data)
	return data.ID
}

func spamChat(wg *sync.WaitGroup, token string, convID int, user string) {
	defer wg.Done()

	// Connect WS
	conn, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("%s?token=%s", WSURL, token), nil)
	if err != nil {
		log.Printf("❌ WS Connect Fail [%s]: %v", user, err)
		return
	}
	defer conn.Close()

	// Spam Loop
	for i := 0; i < MsgCount; i++ {
		msg := map[string]interface{}{
			"conversation_id": convID,
			"content":         fmt.Sprintf("LoadTest Msg %d from %s", i, user),
		}
		err := conn.WriteJSON(msg)
		if err != nil {
			log.Printf("❌ Send Fail [%s]: %v", user, err)
			break
		}
		// Small sleep to prevent instant localhost bottleneck (simulate real network)
		time.Sleep(10 * time.Millisecond)
	}
	log.Printf("✅ %s finished sending %d msgs", user, MsgCount)
}

func postJSON(endpoint string, data interface{}) (*http.Response, error) {
	jsonData, _ := json.Marshal(data)
	return http.Post(BaseURL+endpoint, "application/json", bytes.NewBuffer(jsonData))
}
