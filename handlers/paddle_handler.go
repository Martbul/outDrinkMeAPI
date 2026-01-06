package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
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

type PriceResponse struct {
	ID          string `json:"id"`
	ProductID   string `json:"productId"`
	Description string `json:"description"`
	Amount      string `json:"amount"`
	Currency    string `json:"currency"`
	Interval    string `json:"interval"`
}

func (h *PaddleHandler) GetPrices(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	req := &paddle.ListPricesRequest{
		Status: []string{string(paddle.StatusActive)},
	}

	priceCollection, err := h.paddleService.PaddleClient.ListPrices(ctx, req)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var prices []PriceResponse

	for {
		result := priceCollection.Next(ctx)

		if !result.Ok() {
			if err := result.Err(); err != nil {
				respondWithError(w, http.StatusInternalServerError, err.Error())
				return
			}
			break // Successfully finished iterating
		}

		p := result.Value()

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

	respondWithJSON(w, http.StatusOK, prices)
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

	hostedID := "hsc_01ke920ky3j35wpbw0pqf6cj0t_3ayb30a530yz69681rcc42j3ej6bey5h"

	// Final URL Format: https://sandbox-pay.paddle.io/{ID}?transaction_id={TXN_ID}
	checkoutURL := fmt.Sprintf(
		"https://sandbox-pay.paddle.io/%s?transaction_id=%s",
		hostedID,
		tx.ID,
	)

	fmt.Printf("--- PADDLE DEBUG ---\n")
	fmt.Printf("Transaction Created: %s\n", tx.ID)
	fmt.Printf("Opening URL: %s\n", checkoutURL)

	respondWithJSON(w, http.StatusOK, map[string]string{
		"transactionId": tx.ID,
		"checkoutUrl":   checkoutURL,
	})
}

func (h *PaddleHandler) PaddleWebhookHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	secret := os.Getenv("PADDLE_WEBHOOK_SECRET")
	if secret == "" {
		log.Println("PADDLE_WEBHOOK_SECRET missing")
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

	case paddle.EventTypeNameTransactionPaid:
		type TransactionEvent struct {
			Data paddle.Transaction `json:"data"`
		}
		var fullEvent TransactionEvent
		if err := json.Unmarshal(bodyBytes, &fullEvent); err != nil {
			log.Printf("Error parsing transaction: %v", err)
			return
		}

		entityID = fullEvent.Data.ID

		// Extract User ID
		var userID string
		if fullEvent.Data.CustomData != nil {
			if uid, ok := fullEvent.Data.CustomData["userId"].(string); ok {
				userID = uid
			}
		}

		if userID != "" {
			// 1. Calculate Validity
			monthsToAdd := 1

			// FIX 1: Removed 'Price != nil' check
			// We only check if Items has elements. Price is a struct, so safe to access.
			if len(fullEvent.Data.Items) > 0 {
				desc := strings.ToLower(fullEvent.Data.Items[0].Price.Description)
				if strings.Contains(desc, "year") || strings.Contains(desc, "annual") {
					monthsToAdd = 12
				}
			}
			validUntil := time.Now().AddDate(0, monthsToAdd, 0)

			// 2. Extract Additional Info
			transactionID := fullEvent.Data.ID
			customerID := ""
			if fullEvent.Data.CustomerID != nil {
				customerID = *fullEvent.Data.CustomerID
			}

			// FIX 2: Removed 'Details != nil' and 'Totals != nil' checks
			// These are structs, so we access fields directly.
			// GrandTotal is a string field inside the nested structs.
			amount := fullEvent.Data.Details.Totals.GrandTotal
			currency := string(fullEvent.Data.CurrencyCode)

			fmt.Printf("✅ Payment: %s | User: %s | Amount: %s %s\n", transactionID, userID, amount, currency)

			// 3. Call Service
			err := h.paddleService.UnlockPremium(ctx, userID, validUntil, transactionID, customerID, amount, currency)
			if err != nil {
				log.Printf("❌ Failed to unlock premium: %v", err)
				http.Error(w, "DB Error", http.StatusInternalServerError)
				return
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

func (h *PaddleHandler) HandlePaymentSuccess(w http.ResponseWriter, r *http.Request) {
	html := `
    <!DOCTYPE html>
    <html>
    <head>
        <title>Payment Successful</title>
        <meta name="viewport" content="width=device-width, initial-scale=1">
    </head>
    <body>
        <script>
            setTimeout(function() {
                window.location.href = "outdrinkme://payment-success";
            }, 1);
        </script>
    </body>
    </html>
    `
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}
