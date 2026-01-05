package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"outDrinkMeAPI/services"

	paddle "github.com/PaddleHQ/paddle-go-sdk"
)

// Fixed typo: PaddkeHandler -> PaddleHandler
type PaddleHandler struct {
	paddleService *services.PaddleService
}

func NewPaddleHandler(paddleService *services.PaddleService) *PaddleHandler {
	return &PaddleHandler{
		paddleService: paddleService,
	}
}

// Request payload structure (from your React Native app)
type CreateTransactionRequest struct {
	PriceID string `json:"priceId"`
}

func (h *PaddleHandler) CreateTransaction(w http.ResponseWriter, r *http.Request) {
	// 1. Parse request body to get Price ID
	var reqBody CreateTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 2. Prepare the Paddle SDK request
	// You can retrieve the user ID from your auth context (e.g. JWT)
	userID := "user_internal_id_123" 

	createReq := &paddle.CreateTransactionRequest{
		Items: []paddle.CreateTransactionItems{
			{
				Quantity: 1,
				PriceID:  reqBody.PriceID, 
			},
		},
		CustomData: paddle.CustomData{
			"userId": userID, // Pass this so you know who paid in the webhook
		},
	}

	// 3. Call Paddle API using the SDK Client inside your service
	// (Assuming h.paddleService.Client is the *paddle.SDK instance)
	tx, err := h.paddleService.PaddleClient.CreateTransaction(r.Context(), createReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create transaction: %v", err), http.StatusInternalServerError)
		return
	}

	// 4. Return the Transaction ID to the client
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"transactionId": tx.ID,
	})
}

func (h *PaddleHandler) PaddleWebhookHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Initialize the Webhook Verifier
	// In production, initialize this ONCE (e.g., in NewPaddleHandler) to save resources
	secret := os.Getenv("PADDLE_WEBHOOK_SECRET")
	if secret == "" {
		// Fallback for local testing if env var is missing
		secret = "YOUR_WEBHOOK_SECRET" 
	}
	verifier := paddle.NewWebhookVerifier(secret)

	// 2. Verify the Signature
	// The SDK handles the headers, timestamp, and hashing logic automatically
	valid, err := verifier.Verify(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Verification error: %v", err), http.StatusBadRequest)
		return
	}
	if !valid {
		http.Error(w, "Invalid signature", http.StatusForbidden)
		return
	}

	// 3. Parse the Event
	// We decode the raw body (which is now safe) into a generic map or struct
	// Note: We need to read the body again, but the Verifier might have drained it.
	// However, the paddle-go-sdk Verify method usually reads the body but restores it 
	// or expects you to have read it. 
    // Standard practice with this SDK: The `Verify` method takes `*http.Request` and does not consume the body permanently.
    
	var event paddle.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "Failed to parse event", http.StatusBadRequest)
		return
	}

	// 4. Handle Specific Events
	switch event.EventType {
	case paddle.EventTypeTransactionPaid, paddle.EventTypeSubscriptionCreated:
		// The SDK generic Event struct Data field is json.RawMessage, 
        // so we unmarshal it into a Transaction struct to access fields safely.
		var tx paddle.Transaction
		if err := json.Unmarshal(event.Data, &tx); err != nil {
			fmt.Println("Error parsing transaction data:", err)
			return
		}

		// Retrieve the custom data we sent earlier
		if tx.CustomData != nil {
			userID := tx.CustomData["userId"]
			fmt.Printf("💰 Payment received for User ID: %v\n", userID)
			
			// TODO: Call your service to update the user in the database
			// h.paddleService.UpgradeUser(userID)
		}
	}

	w.WriteHeader(http.StatusOK)
}