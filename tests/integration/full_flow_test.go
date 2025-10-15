package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"outDrinkMeAPI/handlers"
	modelUser "outDrinkMeAPI/internal/user"
	"outDrinkMeAPI/middleware"
	"outDrinkMeAPI/services"
	"outDrinkMeAPI/tests/helpers"
)

// TestFullSignUpAndLoginFlow simulates the complete flow
func TestFullSignUpAndLoginFlow(t *testing.T) {
	// Setup
	pool := helpers.SetupTestDB(t)
	defer helpers.CleanupTestDB(t, pool)

	userService := services.NewUserService(pool)
	userHandler := handlers.NewUserHandler(userService)
	webhookHandler := handlers.NewWebhookHandler(userService)

	os.Setenv("CLERK_WEBHOOK_SECRET", "")
	defer os.Unsetenv("CLERK_WEBHOOK_SECRET")

	clerkID := "user_test_" + time.Now().Format("20060102150405")

	// Step 1: Simulate user signs up with Google via Clerk
	t.Log("Step 1: User signs up with Google")

	createPayload := helpers.MockClerkWebhookPayload("user.created", clerkID)
	req1 := httptest.NewRequest(http.MethodPost, "/webhooks/clerk", bytes.NewReader(createPayload))
	rr1 := httptest.NewRecorder()

	webhookHandler.HandleClerkWebhook(rr1, req1)
	assert.Equal(t, http.StatusOK, rr1.Code, "Webhook should succeed")

	// Step 2: Verify user exists in database
	t.Log("Step 2: Verify user in database")

	ctx := context.Background()
	user, err := userService.GetUserByClerkID(ctx, clerkID)
	require.NoError(t, err)
	assert.Equal(t, "test.user@example.com", user.Email)
	assert.True(t, user.EmailVerified)

	// Step 3: Simulate user logs in and gets profile
	t.Log("Step 3: User gets profile")

	req2 := httptest.NewRequest(http.MethodGet, "/api/user", nil)
	ctx = context.WithValue(req2.Context(), middleware.ClerkIDKey, clerkID)
	req2 = req2.WithContext(ctx)
	rr2 := httptest.NewRecorder()

	userHandler.GetProfile(rr2, req2)
	assert.Equal(t, http.StatusOK, rr2.Code)

	var profile modelUser.User
	err = json.Unmarshal(rr2.Body.Bytes(), &profile)
	require.NoError(t, err)
	assert.Equal(t, user.Email, profile.Email)

	// Step 4: User updates profile
	t.Log("Step 4: User updates profile")

	updateData := `{"firstName": "NewFirst", "username": "newusername123"}`
	req3 := httptest.NewRequest(http.MethodPut, "/api/user/profile", strings.NewReader(updateData))
	req3.Header.Set("Content-Type", "application/json")
	ctx = context.WithValue(req3.Context(), middleware.ClerkIDKey, clerkID)
	req3 = req3.WithContext(ctx)
	rr3 := httptest.NewRecorder()

	userHandler.UpdateProfile(rr3, req3)
	assert.Equal(t, http.StatusOK, rr3.Code)

	// Step 5: Verify update
	t.Log("Step 5: Verify profile update")

	updatedUser, err := userService.GetUserByClerkID(ctx, clerkID)
	require.NoError(t, err)
	assert.Equal(t, "NewFirst", updatedUser.FirstName)
	assert.Equal(t, "newusername123", updatedUser.Username)

	// Step 6: User logs out (no server action needed for Clerk)
	// Step 7: User logs back in (same as step 3)

	// Step 8: User deletes account
	t.Log("Step 6: User deletes account")

	req4 := httptest.NewRequest(http.MethodDelete, "/api/user/account", nil)
	ctx = context.WithValue(req4.Context(), middleware.ClerkIDKey, clerkID)
	req4 = req4.WithContext(ctx)
	rr4 := httptest.NewRecorder()

	userHandler.DeleteAccount(rr4, req4)
	assert.Equal(t, http.StatusOK, rr4.Code)

	// Verify deletion
	_, err = userService.GetUserByClerkID(ctx, clerkID)
	assert.Error(t, err, "User should be deleted")
}
