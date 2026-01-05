package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"outDrinkMeAPI/internal/types/clerk"
	"outDrinkMeAPI/internal/types/subscription"
	"outDrinkMeAPI/internal/types/user"
	"outDrinkMeAPI/services"

	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/webhook"
)

type WebhookHandler struct {
	userService *services.UserService
}

func NewWebhookHandler(userService *services.UserService) *WebhookHandler {
	return &WebhookHandler{
		userService: userService,
	}
}

func (h *WebhookHandler) HandleClerkWebhook(w http.ResponseWriter, r *http.Request) {
	// Verify webhook signature
	if !h.verifyWebhookSignature(r) {
		log.Println("Invalid webhook signature")
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading webhook body: %v", err)
		http.Error(w, "Error reading body", http.StatusBadRequest)
		return
	}

	// Parse event
	var event clerk.ClerkWebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		log.Printf("Error parsing webhook: %v", err)
		http.Error(w, "Error parsing webhook", http.StatusBadRequest)
		return
	}

	log.Printf("Received webhook event: %s", event.Type)

	// Handle different event types
	ctx := r.Context()
	switch event.Type {
	case "user.created":
		if err := h.handleUserCreated(ctx, event.Data); err != nil {
			log.Printf("Error handling user.created: %v", err)
			http.Error(w, "Error processing webhook", http.StatusInternalServerError)
			return
		}

	case "user.updated":
		if err := h.handleUserUpdated(ctx, event.Data); err != nil {
			log.Printf("Error handling user.updated: %v", err)
			http.Error(w, "Error processing webhook", http.StatusInternalServerError)
			return
		}

	case "user.deleted":
		if err := h.handleUserDeleted(ctx, event.Data); err != nil {
			log.Printf("Error handling user.deleted: %v", err)
			http.Error(w, "Error processing webhook", http.StatusInternalServerError)
			return
		}

	case "email.created":
		if err := h.handleEmailVerified(ctx, event.Data); err != nil {
			log.Printf("Error handling email.created: %v", err)
			// Don't return error, this is not critical
		}

	default:
		log.Printf("Unhandled webhook event type: %s", event.Type)
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"success": true}`))
}

func (h *WebhookHandler) handleUserCreated(ctx context.Context, data json.RawMessage) error {
	var userData clerk.ClerkUserData
	if err := json.Unmarshal(data, &userData); err != nil {
		return fmt.Errorf("failed to unmarshal user data: %w", err)
	}

	// Get primary email
	email := ""
	emailVerified := false
	if len(userData.EmailAddresses) > 0 {
		email = userData.EmailAddresses[0].EmailAddress
		emailVerified = userData.EmailAddresses[0].Verification.Status == "verified"
	}

	// Use username or generate from email
	username := userData.Username
	if username == "" {
		username = userData.FirstName + userData.LastName
	}

	// Choose image URL
	imageURL := userData.ImageURL
	if imageURL == "" {
		imageURL = userData.ProfileImageURL
	}

	createReq := &user.CreateUserRequest{
		ClerkID:   userData.ID,
		Email:     email,
		Username:  username,
		FirstName: userData.FirstName,
		LastName:  userData.LastName,
		ImageURL:  imageURL,
	}

	user, err := h.userService.CreateUser(ctx, createReq)
	if err != nil {
		return fmt.Errorf("failed to create user in database: %w", err)
	}

	// Update email verification status
	if emailVerified {
		h.userService.UpdateEmailVerification(ctx, userData.ID, true)
	}

	log.Printf("Successfully created user: %s (Clerk ID: %s)", user.Email, user.ClerkID)
	return nil
}

func (h *WebhookHandler) handleUserUpdated(ctx context.Context, data json.RawMessage) error {
	var userData clerk.ClerkUserData
	if err := json.Unmarshal(data, &userData); err != nil {
		return fmt.Errorf("failed to unmarshal user data: %w", err)
	}

	// Use username or generate from name
	username := userData.Username
	if username == "" {
		username = userData.FirstName + userData.LastName
	}

	imageURL := userData.ImageURL
	if imageURL == "" {
		imageURL = userData.ProfileImageURL
	}

	updateReq := &user.UpdateProfileRequest{
		Username:  username,
		FirstName: userData.FirstName,
		LastName:  userData.LastName,
		ImageURL:  imageURL,
	}

	_, err := h.userService.UpdateProfileByClerkID(ctx, userData.ID, updateReq)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	log.Printf("Successfully updated user: Clerk ID: %s", userData.ID)
	return nil
}

func (h *WebhookHandler) handleUserDeleted(ctx context.Context, data json.RawMessage) error {
	var userData struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &userData); err != nil {
		return fmt.Errorf("failed to unmarshal user data: %w", err)
	}

	if err := h.userService.DeleteUserByClerkID(ctx, userData.ID); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	log.Printf("Successfully deleted user: Clerk ID: %s", userData.ID)
	return nil
}

func (h *WebhookHandler) handleEmailVerified(ctx context.Context, data json.RawMessage) error {
	var emailData struct {
		ID     string `json:"id"`
		Object string `json:"object"`
	}
	if err := json.Unmarshal(data, &emailData); err != nil {
		return fmt.Errorf("failed to unmarshal email data: %w", err)
	}

	// Note: You might need to fetch the user ID from Clerk API here
	// For now, we'll skip this implementation
	log.Printf("Email verified event received: %s", emailData.ID)
	return nil
}

func (h *WebhookHandler) verifyWebhookSignature(r *http.Request) bool {
	webhookSecret := os.Getenv("CLERK_WEBHOOK_SECRET")
	if webhookSecret == "" {
		log.Println("CLERK_WEBHOOK_SECRET not set, skipping signature verification")
		return true // In development, you might want to skip verification
	}

	// Get signature from headers
	svixID := r.Header.Get("svix-id")
	svixTimestamp := r.Header.Get("svix-timestamp")
	svixSignature := r.Header.Get("svix-signature")

	if svixID == "" || svixTimestamp == "" || svixSignature == "" {
		log.Println("Missing webhook signature headers")
		return false
	}

	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading body for verification: %v", err)
		return false
	}

	// Create signed content
	signedContent := fmt.Sprintf("%s.%s.%s", svixID, svixTimestamp, string(body))

	// Calculate expected signature
	mac := hmac.New(sha256.New, []byte(webhookSecret))
	mac.Write([]byte(signedContent))
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	// Compare signatures (v1 format)
	providedSignature := ""
	if len(svixSignature) > 3 && svixSignature[:3] == "v1," {
		providedSignature = svixSignature[3:]
	}

	return hmac.Equal([]byte(expectedSignature), []byte(providedSignature))
}

// HandleStripeWebhook processes events sent by Stripe
func (h *WebhookHandler) HandleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	const MaxBodyBytes = int64(65536)
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)

	payload, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	// 1. Verify the signature
	endpointSecret := os.Getenv("STRIPE_WEBHOOK_SECRET")
	if endpointSecret == "" {
		log.Println("STRIPE_WEBHOOK_SECRET is not set")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	event, err := webhook.ConstructEvent(payload, r.Header.Get("Stripe-Signature"), endpointSecret)
	if err != nil {
		log.Printf("Error verifying webhook signature: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// 2. Handle specific event types
	ctx := r.Context()

	switch event.Type {
	case "checkout.session.completed":
		// User successfully paid for a subscription
		var session stripe.CheckoutSession
		err := json.Unmarshal(event.Data.Raw, &session)
		if err != nil {
			log.Printf("Error parsing webhook JSON: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if err := h.handleCheckoutSessionCompleted(ctx, &session); err != nil {
			log.Printf("Error handling checkout.session.completed: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

	case "customer.subscription.updated", "customer.subscription.deleted":
		// Subscription renewed, cancelled, or expired
		var sub stripe.Subscription
		err := json.Unmarshal(event.Data.Raw, &sub)
		if err != nil {
			log.Printf("Error parsing webhook JSON: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if err := h.handleSubscriptionUpdated(ctx, &sub); err != nil {
			log.Printf("Error handling subscription update: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

	case "invoice.payment_succeeded":
		// Recurring payment succeeded - we usually use this or subscription.updated
		// to extend the access date. We will fetch the sub and update it.
		var invoice stripe.Invoice
		err := json.Unmarshal(event.Data.Raw, &invoice)
		if err != nil {
			log.Printf("Error parsing webhook JSON: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if invoice.Subscription != nil {
			// We need to fetch the latest subscription data to get the new CurrentPeriodEnd
			// Alternatively, we could extract it from invoice lines, but fetching is safer.
			if err := h.handleInvoicePaymentSucceeded(ctx, invoice.Subscription.ID); err != nil {
				log.Printf("Error handling invoice payment: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (h *WebhookHandler) handleCheckoutSessionCompleted(ctx context.Context, session *stripe.CheckoutSession) error {
	userID := session.Metadata["user_id"]
	if userID == "" {
		return fmt.Errorf("no user_id found in session metadata")
	}

	sub, err := h.userService.FetchStripeSubscription(session.Subscription.ID)
	if err != nil {
		return err
	}

	dbSub := &subscription.Subscription{
		UserID:               userID,
		StripeCustomerID:     session.Customer.ID,
		StripeSubscriptionID: sub.ID,
		StripePriceID:        sub.Items.Data[0].Price.ID,
		Status:               string(sub.Status),
		CurrentPeriodEnd:     time.Unix(sub.CurrentPeriodEnd, 0),
	}

	return h.userService.UpsertSubscription(ctx, dbSub)
}

func (h *WebhookHandler) handleSubscriptionUpdated(ctx context.Context, sub *stripe.Subscription) error {
	// We need to find the user_id from our DB based on the stripe_subscription_id
	// or we can rely on the fact that Upsert will match on stripe_subscription_id.

	dbSub := &subscription.Subscription{
		StripeSubscriptionID: sub.ID,
		StripeCustomerID:     sub.Customer.ID,
		StripePriceID:        sub.Items.Data[0].Price.ID,
		Status:               string(sub.Status),
		CurrentPeriodEnd:     time.Unix(sub.CurrentPeriodEnd, 0),
	}

	return h.userService.UpdateSubscriptionStatus(ctx, dbSub)
}

func (h *WebhookHandler) handleInvoicePaymentSucceeded(ctx context.Context, subscriptionID string) error {
	// Fetch latest data from Stripe
	sub, err := h.userService.FetchStripeSubscription(subscriptionID)
	if err != nil {
		return err
	}

	// Update local DB
	dbSub := &subscription.Subscription{
		StripeSubscriptionID: sub.ID,
		StripeCustomerID:     sub.Customer.ID,
		StripePriceID:        sub.Items.Data[0].Price.ID,
		Status:               string(sub.Status),
		CurrentPeriodEnd:     time.Unix(sub.CurrentPeriodEnd, 0),
	}

	return h.userService.UpdateSubscriptionStatus(ctx, dbSub)
}
