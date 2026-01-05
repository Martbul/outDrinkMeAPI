package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"outDrinkMeAPI/middleware"
	"outDrinkMeAPI/services"

	paddle "github.com/PaddleHQ/paddle-go-sdk"
)

type PaddleHandler struct {
	paddleService *services.PaddleService
}

func NewPaddleHandler(paddleService *services.PaddleService) *PaddleHandler {
	return &PaddleHandler{
		paddleService: paddleService,
	}
}

type CreateTransactionRequest struct {
	PriceID string `json:"priceId"`
}

func (h *PaddleHandler) CreateTransaction(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	var reqBody CreateTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	createReq := &paddle.CreateTransactionRequest{
		Items: []paddle.CreateTransactionItems{
			*paddle.NewCreateTransactionItemsCatalogItem(&paddle.CatalogItem{
				Quantity: 1,
				PriceID:  reqBody.PriceID,
			}),
		},
		CustomData: paddle.CustomData{
			"userId": clerkID,
		},
		CollectionMode: paddle.PtrTo(paddle.CollectionModeAutomatic),
	}

	tx, err := h.paddleService.PaddleClient.CreateTransaction(ctx, createReq)
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


//! unlock and remove premium in the db with service call
func (h *PaddleHandler) PaddleWebhookHandler(w http.ResponseWriter, r *http.Request) {
	secret := os.Getenv("PADDLE_SECRET_KEY")
	if secret == "" {
		log.Println("PADDLE_SECRET_KEY missing")
		http.Error(w, "Configuration Error", http.StatusInternalServerError)
		return
	}

	verifier := paddle.NewWebhookVerifier(secret)

	// Note: Verify typically reads the body. Ensure your SDK version
	// restores the body buffer so it can be read again below.
	valid, err := verifier.Verify(r)
	if err != nil {
		http.Error(w, "Verification failed", http.StatusInternalServerError)
		return
	}
	if !valid {
		http.Error(w, "Invalid signature", http.StatusForbidden)
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Unable to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// FIX 1: Use EventTypeName (string) instead of EventType (struct)
	type WebhookPartial struct {
		EventID   string               `json:"event_id"`
		EventType paddle.EventTypeName `json:"event_type"` 
	}

	var webhook WebhookPartial
	if err := json.Unmarshal(bodyBytes, &webhook); err != nil {
		http.Error(w, "Unable to parse JSON", http.StatusBadRequest)
		return
	}

	var entityID string

	// FIX 2: Use the EventTypeName constants
	switch webhook.EventType {

	// Note: Ensure you use the constants exactly as defined in the SDK (e.g., EventTypeNameTransactionPaid)
	case paddle.EventTypeNameTransactionPaid, paddle.EventTypeNameSubscriptionCreated:
		type TransactionEvent struct {
			Data paddle.Transaction `json:"data"`
		}

		var fullEvent TransactionEvent
		if err := json.Unmarshal(bodyBytes, &fullEvent); err != nil {
			log.Printf("Error parsing transaction: %v", err)
			return
		}

		entityID = fullEvent.Data.ID

		if fullEvent.Data.CustomData != nil {
			if userID, ok := fullEvent.Data.CustomData["userId"].(string); ok {
				fmt.Printf("✅ Payment Succeeded for User: %s\n", userID)
				// h.paddleService.UnlockPremium(userID)
			}
		}

	case paddle.EventTypeNameSubscriptionUpdated:
		type SubscriptionEvent struct {
			Data paddle.Subscription `json:"data"`
		}

		var fullEvent SubscriptionEvent
		if err := json.Unmarshal(bodyBytes, &fullEvent); err != nil {
			log.Printf("Error parsing subscription: %v", err)
			return
		}
		entityID = fullEvent.Data.ID
		fmt.Printf("Subscription updated: %s\n", entityID)

	default:
		entityID = webhook.EventID
		// Cast string(webhook.EventType) for printing if needed
		fmt.Printf("Unhandled event type: %s\n", webhook.EventType)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf(`{"ID": "%s"}`, entityID)))
}