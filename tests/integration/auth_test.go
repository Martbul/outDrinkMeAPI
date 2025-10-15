package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"outDrinkMeAPI/handlers"
	"outDrinkMeAPI/internal/user"
	"outDrinkMeAPI/middleware"
	"outDrinkMeAPI/services"
	"outDrinkMeAPI/tests/helpers"
)

func TestGetProfile_Authenticated(t *testing.T) {
	// Setup
	pool := helpers.SetupTestDB(t)
	defer helpers.CleanupTestDB(t, pool)

	userService := services.NewUserService(pool)
	userHandler := handlers.NewUserHandler(userService)

	// Create a test user
	clerkID := "user_test_" + time.Now().Format("20060102150405")
	ctx := context.Background()
	
	createReq := &user.CreateUserRequest{
		ClerkID:   clerkID,
		Email:     "testauth@example.com",
		Username:  "testauth",
		FirstName: "Test",
		LastName:  "Auth",
		ImageURL:  "https://example.com/image.jpg",
	}
	
	createdUser, err := userService.CreateUser(ctx, createReq)
	require.NoError(t, err)

	// Mock JWT verification by setting up context
	// In real scenario, you'd need to mock jwt.Verify
	// For this test, we'll manually add user to context
	
	// Create request with auth
	req := httptest.NewRequest(http.MethodGet, "/api/user", nil)
	
	// Add clerk ID to context (simulating successful auth middleware)
	ctx = context.WithValue(req.Context(), middleware.ClerkIDKey, clerkID)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	// Execute
	userHandler.GetProfile(rr, req)

	// Assert
	assert.Equal(t, http.StatusOK, rr.Code)

	var response user.User
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, createdUser.ID, response.ID)
	assert.Equal(t, clerkID, response.ClerkID)
	assert.Equal(t, "testauth@example.com", response.Email)
	assert.Equal(t, "testauth", response.Username)
}

func TestGetProfile_Unauthenticated(t *testing.T) {
	// Setup
	pool := helpers.SetupTestDB(t)
	defer helpers.CleanupTestDB(t, pool)

	userService := services.NewUserService(pool)
	userHandler := handlers.NewUserHandler(userService)

	// Create request WITHOUT auth
	req := httptest.NewRequest(http.MethodGet, "/api/user", nil)
	rr := httptest.NewRecorder()

	// Execute
	userHandler.GetProfile(rr, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, rr.Code)

	var response map[string]string
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["error"], "not authenticated")
}

func TestUpdateProfile_Authenticated(t *testing.T) {
	// Setup
	pool := helpers.SetupTestDB(t)
	defer helpers.CleanupTestDB(t, pool)

	userService := services.NewUserService(pool)
	userHandler := handlers.NewUserHandler(userService)

	// Create a test user
	clerkID := "user_test_" + time.Now().Format("20060102150405")
	ctx := context.Background()
	
	_, err := userService.CreateUser(ctx, &user.CreateUserRequest{
		ClerkID:   clerkID,
		Email:     "testupdate@example.com",
		Username:  "testupdate",
		FirstName: "Test",
		LastName:  "Update",
	})
	require.NoError(t, err)

	// Create update request
	updateData := `{"firstName": "Updated", "lastName": "Name", "username": "newusername"}`
	req := httptest.NewRequest(http.MethodPut, "/api/user/profile", strings.NewReader(updateData))
	req.Header.Set("Content-Type", "application/json")

	// Add auth context
	ctx = context.WithValue(req.Context(), middleware.ClerkIDKey, clerkID)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	// Execute
	userHandler.UpdateProfile(rr, req)

	// Assert
	assert.Equal(t, http.StatusOK, rr.Code)

	var response user.User
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Updated", response.FirstName)
	assert.Equal(t, "Name", response.LastName)
	assert.Equal(t, "newusername", response.Username)
}

func TestDeleteAccount_Authenticated(t *testing.T) {
	// Setup
	pool := helpers.SetupTestDB(t)
	defer helpers.CleanupTestDB(t, pool)

	userService := services.NewUserService(pool)
	userHandler := handlers.NewUserHandler(userService)

	// Create a test user
	clerkID := "user_test_" + time.Now().Format("20060102150405")
	ctx := context.Background()
	
	_, err := userService.CreateUser(ctx, &user.CreateUserRequest{
		ClerkID:   clerkID,
		Email:     "testdelete@example.com",
		Username:  "testdelete",
		FirstName: "Test",
		LastName:  "Delete",
	})
	require.NoError(t, err)

	// Create delete request
	req := httptest.NewRequest(http.MethodDelete, "/api/user/account", nil)

	// Add auth context
	ctx = context.WithValue(req.Context(), middleware.ClerkIDKey, clerkID)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	// Execute
	userHandler.DeleteAccount(rr, req)

	// Assert
	assert.Equal(t, http.StatusOK, rr.Code)

	// Verify deletion
	_, err = userService.GetUserByClerkID(ctx, clerkID)
	assert.Error(t, err, "User should be deleted")
}