package myMiddleware

import (
	"context"
	"net/http"
	"strings"
)

// 1. Define Context Keys (Exported so other packages can read them)
type contextKey string

const (
	UserKey     contextKey = "user_id"
	UsernameKey contextKey = "username"
)

// 2. Define what we need from the User Service
// This interface decouples 'middleware' from 'user'
type TokenValidator interface {
	ValidateToken(tokenString string) (int, string, error)
}

// 3. The Middleware Structure
type AuthMiddleware struct {
	validator TokenValidator
}

func NewAuthMiddleware(v TokenValidator) *AuthMiddleware {
	return &AuthMiddleware{validator: v}
}

// 4. The actual Handler
func (am *AuthMiddleware) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString := ""

		// Check Authorization Header
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 {
				tokenString = parts[1]
			}
		}

		// Fallback: Check Query Param
		if tokenString == "" {
			tokenString = r.URL.Query().Get("token")
		}

		if tokenString == "" {
			http.Error(w, "Missing authentication token", http.StatusUnauthorized)
			return
		}

		// Validate using the interface
		userID, username, err := am.validator.ValidateToken(tokenString)
		if err != nil {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Inject into Context
		ctx := context.WithValue(r.Context(), UserKey, userID)
		ctx = context.WithValue(ctx, UsernameKey, username)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
