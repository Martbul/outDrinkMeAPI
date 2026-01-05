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

// Response struct for sending prices to the client
type PriceResponse struct {
	ID          string `json:"id"`
	ProductID   string `json:"productId"`
	Description string `json:"description"`
	Amount      string `json:"amount"`
	Currency    string `json:"currency"`
	Interval    string `json:"interval"` // e.g., "month", "year"
}

func (h *PaddleHandler) GetPrices(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// FIX 1 & 2: Use paddle.Status and paddle.StatusActive
	req := &paddle.ListPricesRequest{
		Status: []string{string(paddle.StatusActive)},
	}
	
	priceCollection, err := h.paddleService.PaddleClient.ListPrices(ctx, req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch prices: %v", err), http.StatusInternalServerError)
		return
	}

	var prices []PriceResponse

	// FIX 3: Iterate using the SDK's Collection iterator
	for {
		// Get the next result wrapper
		result := priceCollection.Next(ctx)

		// If !Ok(), we are done or there was an error
		if !result.Ok() {
			if err := result.Err(); err != nil {
				// Handle error (e.g., network failure fetching next page)
				http.Error(w, fmt.Sprintf("Error iterating prices: %v", err), http.StatusInternalServerError)
				return
			}
			break // Successfully finished iterating
		}

		// Extract the actual price pointer
		p := result.Value()

		// Logic to extract interval safely
		interval := ""
		if p.BillingCycle != nil {
			interval = string(p.BillingCycle.Interval)
		}

		prices = append(prices, PriceResponse{
			ID:          p.ID,
			ProductID:   p.ProductID,
			Description: p.Description,
			Amount:      p.UnitPrice.Amount,
			Currency:    string(p.UnitPrice.CurrencyCode),
			Interval:    interval,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(prices)
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

// ! unlock and remove premium in the db with service call
func (h *PaddleHandler) PaddleWebhookHandler(w http.ResponseWriter, r *http.Request) {
	secret := os.Getenv("PADDLE_SECRET_KEY")
	if secret == "" {
		log.Println("PADDLE_SECRET_KEY missing")
		http.Error(w, "Configuration Error", http.StatusInternalServerError)
		return
	}

	verifier := paddle.NewWebhookVerifier(secret)

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

	switch webhook.EventType {

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
				h.paddleService.UnlockPremium(userID)
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
