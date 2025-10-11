package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"outDrinkMeAPI/handlers"
	"outDrinkMeAPI/services"
	"outDrinkMeAPI/tests/helpers"
)

func TestWebhookUserCreated(t *testing.T) {
	// Setup
	pool := helpers.SetupTestDB(t)
	defer helpers.CleanupTestDB(t, pool)

	userService := services.NewUserService(pool)
	webhookHandler := handlers.NewWebhookHandler(userService)

	// Disable signature verification for testing
	os.Setenv("CLERK_WEBHOOK_SECRET", "")
	defer os.Unsetenv("CLERK_WEBHOOK_SECRET")

	// Test data
	clerkID := "user_test_" + time.Now().Format("20060102150405")
	payload := helpers.MockClerkWebhookPayload("user.created", clerkID)

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/webhooks/clerk", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	
	// Create response recorder
	rr := httptest.NewRecorder()

	// Execute
	webhookHandler.HandleClerkWebhook(rr, req)

	// Assert response
	assert.Equal(t, http.StatusOK, rr.Code, "Expected status 200")

	var response map[string]bool
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response["success"])

	// Verify user was created in database
	ctx := context.Background()
	user, err := userService.GetUserByClerkID(ctx, clerkID)
	require.NoError(t, err, "User should be created")
	assert.Equal(t, clerkID, user.ClerkID)
	assert.Equal(t, "test.user@example.com", user.Email)
	assert.Equal(t, "Test", user.FirstName)
	assert.Equal(t, "User", user.LastName)
	assert.True(t, user.EmailVerified, "Email should be verified for Google OAuth users")
}

func TestWebhookUserUpdated(t *testing.T) {
	// Setup
	pool := helpers.SetupTestDB(t)
	defer helpers.CleanupTestDB(t, pool)

	userService := services.NewUserService(pool)
	webhookHandler := handlers.NewWebhookHandler(userService)

	os.Setenv("CLERK_WEBHOOK_SECRET", "")
	defer os.Unsetenv("CLERK_WEBHOOK_SECRET")

	// Create user first
	clerkID := "user_test_" + time.Now().Format("20060102150405")
	createPayload := helpers.MockClerkWebhookPayload("user.created", clerkID)
	
	req1 := httptest.NewRequest(http.MethodPost, "/webhooks/clerk", bytes.NewReader(createPayload))
	req1.Header.Set("Content-Type", "application/json")
	rr1 := httptest.NewRecorder()
	webhookHandler.HandleClerkWebhook(rr1, req1)
	require.Equal(t, http.StatusOK, rr1.Code)

	// Update user
	updatePayload := helpers.MockClerkWebhookPayload("user.updated", clerkID)
	req2 := httptest.NewRequest(http.MethodPost, "/webhooks/clerk", bytes.NewReader(updatePayload))
	req2.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()

	webhookHandler.HandleClerkWebhook(rr2, req2)

	// Assert
	assert.Equal(t, http.StatusOK, rr2.Code)

	// Verify update
	ctx := context.Background()
	user, err := userService.GetUserByClerkID(ctx, clerkID)
	require.NoError(t, err)
	assert.Equal(t, "Updated", user.FirstName)
	assert.Equal(t, "updateduser", user.Username)
}

func TestWebhookUserDeleted(t *testing.T) {
	// Setup
	pool := helpers.SetupTestDB(t)
	defer helpers.CleanupTestDB(t, pool)

	userService := services.NewUserService(pool)
	webhookHandler := handlers.NewWebhookHandler(userService)

	os.Setenv("CLERK_WEBHOOK_SECRET", "")
	defer os.Unsetenv("CLERK_WEBHOOK_SECRET")

	// Create user first
	clerkID := "user_test_" + time.Now().Format("20060102150405")
	createPayload := helpers.MockClerkWebhookPayload("user.created", clerkID)
	
	req1 := httptest.NewRequest(http.MethodPost, "/webhooks/clerk", bytes.NewReader(createPayload))
	rr1 := httptest.NewRecorder()
	webhookHandler.HandleClerkWebhook(rr1, req1)

	// Delete user
	deletePayload := helpers.MockClerkWebhookPayload("user.deleted", clerkID)
	req2 := httptest.NewRequest(http.MethodPost, "/webhooks/clerk", bytes.NewReader(deletePayload))
	rr2 := httptest.NewRecorder()

	webhookHandler.HandleClerkWebhook(rr2, req2)

	// Assert
	assert.Equal(t, http.StatusOK, rr2.Code)

	// Verify deletion
	ctx := context.Background()
	_, err := userService.GetUserByClerkID(ctx, clerkID)
	assert.Error(t, err, "User should be deleted")
}
