package middleware

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/clerk/clerk-sdk-go/v2/jwt"
)

type contextKey string

const UserIDKey contextKey = "userID"
const ClerkIDKey contextKey = "clerkID"

// ClerkAuthMiddleware validates Clerk JWT tokens and extracts user info
func ClerkAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			respondWithError(w, http.StatusUnauthorized, "Authorization header required")
			return
		}

		// Remove "Bearer " prefix
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == authHeader {
			respondWithError(w, http.StatusUnauthorized, "Invalid authorization format. Use 'Bearer <token>'")
			return
		}

		// Verify the token
		claims, err := jwt.Verify(r.Context(), &jwt.VerifyParams{
			Token: token,
		})
		if err != nil {
			log.Printf("Token verification failed: %v", err)
			respondWithError(w, http.StatusUnauthorized, fmt.Sprintf("Invalid token: %v", err))
			return
		}

		// Add Clerk user ID to context
		ctx := context.WithValue(r.Context(), ClerkIDKey, claims.Subject)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// OptionalAuthMiddleware - allows requests with or without auth
func OptionalAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			token := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := jwt.Verify(r.Context(), &jwt.VerifyParams{
				Token: token,
			})
			if err == nil {
				ctx := context.WithValue(r.Context(), ClerkIDKey, claims.Subject)
				r = r.WithContext(ctx)
			}
		}
		next.ServeHTTP(w, r)
	})
}

// GetClerkID extracts Clerk user ID from context
func GetClerkID(ctx context.Context) (string, bool) {
	clerkID, ok := ctx.Value(ClerkIDKey).(string)
	return clerkID, ok
}

// GetUserID extracts internal user ID from context
func GetUserID(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(UserIDKey).(string)
	return userID, ok
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write([]byte(fmt.Sprintf(`{"error": "%s"}`, message)))
}
