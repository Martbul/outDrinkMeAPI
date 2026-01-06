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

	var checkoutUrl string = "outdrinkme://payment-success"

	// Build the request
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
		// IMPORTANT: Set this to Automatic to ensure Paddle handles the billing
		CollectionMode: paddle.PtrTo(paddle.CollectionModeAutomatic),
		


		Checkout: &paddle.TransactionCheckout{
			URL: &checkoutUrl,
		},
	}

	tx, err := h.paddleService.PaddleClient.CreateTransaction(ctx, createReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create transaction: %v", err), http.StatusInternalServerError)
		return
	}

	// DEBUG: Log the transaction ID to your terminal
	fmt.Printf("Created Transaction: %s Status: %s\n", tx.ID, tx.Status)

	// If the transaction is ALREADY "billed", it will skip payment.
	// It should be "ready". If it's "billed", check your Price settings in Paddle.

	paddleEnv := "sandbox-checkout" // Switch to "checkout" for production
	// We use the 'custom' checkout endpoint which is designed for Transaction IDs
	checkoutURL := fmt.Sprintf("https://%s.paddle.com/checkout/custom?_ptxn=%s", paddleEnv, tx.ID)

	response := map[string]string{
		"transactionId": tx.ID,
		"checkoutUrl":   checkoutURL,
	}

	respondWithJSON(w, http.StatusOK, response)
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

func (h *PaddleHandler) PaymentSuccessPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	html := `
	<!DOCTYPE html>
	<html>
	<head>
		<title>Payment Successful</title>
		<meta name="viewport" content="width=device-width, initial-scale=1">
		<style>
			body { background-color: #121212; color: white; font-family: sans-serif; text-align: center; padding: 50px 20px; }
			h1 { color: #EA580C; }
			p { color: #888; }
			.card { background: #1E1E1E; padding: 30px; border-radius: 15px; max-width: 400px; margin: 0 auto; }
		</style>
	</head>
	<body>
		<div class="card">
			<h1>Payment Successful!</h1>
			<p>Thank you for subscribing to OutDrinkMe Premium.</p>
			<p>You can now close this window and return to the app.</p>
		</div>
	</body>
	</html>
	`
	fmt.Fprint(w, html)
}
