package helpers

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestDBPool creates a test database connection
func SetupTestDB(t *testing.T) *pgxpool.Pool {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = os.Getenv("DATABASE_URL")
	}
	if dbURL == "" {
		t.Fatal("TEST_DATABASE_URL or DATABASE_URL must be set")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("Failed to ping test database: %v", err)
	}

	return pool
}

// CleanupTestDB cleans up test data
func CleanupTestDB(t *testing.T, pool *pgxpool.Pool) {
	ctx := context.Background()
	_, err := pool.Exec(ctx, "DELETE FROM users WHERE email LIKE 'test%@example.com'")
	if err != nil {
		t.Logf("Warning: failed to cleanup test data: %v", err)
	}
	pool.Close()
}

// GenerateMockClerkJWT generates a mock JWT token for testing
func GenerateMockClerkJWT(clerkID string) (string, error) {
	// Use a test secret key
	secretKey := []byte("test-secret-key-for-testing-only")

	claims := jwt.MapClaims{
		"sub": clerkID,                                    // Clerk user ID
		"iss": "https://clerk.test",                       // Issuer
		"iat": time.Now().Unix(),                          // Issued at
		"exp": time.Now().Add(time.Hour * 24).Unix(),      // Expires in 24 hours
		"azp": "test-app-id",                              // Authorized party
		"sid": "sess_test123",                             // Session ID
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(secretKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// MockClerkWebhookPayload creates a mock webhook payload
func MockClerkWebhookPayload(eventType string, clerkID string) []byte {
	payload := ""
	
	switch eventType {
	case "user.created":
		payload = fmt.Sprintf(`{
			"data": {
				"id": "%s",
				"first_name": "Test",
				"last_name": "User",
				"email_addresses": [{
					"id": "email_123",
					"email_address": "test.user@example.com",
					"verification": {"status": "verified"}
				}],
				"primary_email_address_id": "email_123",
				"username": "testuser",
				"image_url": "https://example.com/image.jpg",
				"profile_image_url": "https://example.com/image.jpg",
				"external_accounts": [{
					"provider": "oauth_google",
					"email_address": "test.user@example.com",
					"first_name": "Test",
					"last_name": "User",
					"picture": "https://example.com/google.jpg",
					"verified_at": "2025-10-11T12:00:00.000Z",
					"google_id": "1234567890"
				}]
			},
			"object": "event",
			"type": "%s"
		}`, clerkID, eventType)
	
	case "user.updated":
		payload = fmt.Sprintf(`{
			"data": {
				"id": "%s",
				"first_name": "Updated",
				"last_name": "User",
				"email_addresses": [{
					"id": "email_123",
					"email_address": "test.user@example.com",
					"verification": {"status": "verified"}
				}],
				"username": "updateduser",
				"image_url": "https://example.com/new-image.jpg",
				"external_accounts": [{
					"provider": "oauth_google",
					"email_address": "test.user@example.com",
					"first_name": "Updated",
					"last_name": "User",
					"picture": "https://example.com/new-google.jpg"
				}]
			},
			"object": "event",
			"type": "%s"
		}`, clerkID, eventType)
	
	case "user.deleted":
		payload = fmt.Sprintf(`{
			"data": {
				"id": "%s",
				"deleted": true
			},
			"object": "event",
			"type": "%s"
		}`, clerkID, eventType)
	}

	return []byte(payload)
}