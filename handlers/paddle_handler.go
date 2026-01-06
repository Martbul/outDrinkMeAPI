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

	// 1. Get Clerk ID
	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// 2. Decode Request
	var reqBody CreateTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 3. Build Transaction
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

	// 4. MANUALLY CONSTRUCT THE HOSTED CHECKOUT URL
	// We use the Hosted Checkout ID you provided in Step 1
	hostedCheckoutID := "hsc_01ke920ky3j35wpbw0pqf6cj0t_3ayb30a530yz69681rcc42j3ej6bey5h"
	
	// Format: https://sandbox-pay.paddle.io/checkout/{ID}?transaction_id={TXN_ID}
	checkoutURL := fmt.Sprintf(
		"https://sandbox-pay.paddle.io/checkout/%s?transaction_id=%s",
		hostedCheckoutID,
		tx.ID,
	)

	// LOGS: Verify these in your Render/Server logs
	fmt.Printf("--- PADDLE SUCCESS ---\n")
	fmt.Printf("Transaction: %s\n", tx.ID)
	fmt.Printf("Final Checkout Link: %s\n", checkoutURL)
	fmt.Printf("----------------------\n")

	// 5. Send to React Native
	respondWithJSON(w, http.StatusOK, map[string]string{
		"transactionId": tx.ID,
		"checkoutUrl":   checkoutURL,
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

func (h *PaddleHandler) HandlePaymentSuccess(w http.ResponseWriter, r *http.Request) {
	html := `
    <!DOCTYPE html>
    <html>
    <head>
        <title>Payment Successful</title>
        <meta name="viewport" content="width=device-width, initial-scale=1">
        <style>
            body { background: #000; color: #fff; font-family: sans-serif; display: flex; 
                   flex-direction: column; align-items: center; justify-content: center; height: 100vh; margin: 0; }
            .loader { border: 4px solid #333; border-top: 4px solid #EA580C; border-radius: 50%; 
                      width: 40px; height: 40px; animation: spin 1s linear infinite; margin-bottom: 20px; }
            @keyframes spin { 0% { transform: rotate(0deg); } 100% { transform: rotate(360deg); } }
        </style>
    </head>
    <body>
        <div class="loader"></div>
        <h2 style="color: #EA580C">Payment Verified</h2>
        <p>Returning you to OutDrinkMe...</p>

        <script>
            // This is the magic: It forces the mobile device to open your app
            setTimeout(function() {
                window.location.href = "outdrinkme://payment-success";
            }, 1500);
        </script>
    </body>
    </html>
    `
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}
