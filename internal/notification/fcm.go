package notification

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"outDrinkMeAPI/internal/types/notification"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

type FCMService struct {
	client *messaging.Client
}

// NewFCMService initializes FCMService. It first attempts to use
// credentials from the FCM_SERVICE_ACCOUNT_JSON environment variable (Base64 encoded).
// If that's not found, it falls back to a local service account key file.
func NewFCMService(localFilePath string) (*FCMService, error) {
	var opt option.ClientOption

	// Attempt to get Base64 encoded credentials from environment variable
	encodedCreds := os.Getenv("FCM_SERVICE_ACCOUNT_JSON") // Corrected env var name
	if encodedCreds != "" {
		decoded, err := base64.StdEncoding.DecodeString(encodedCreds)
		if err != nil {
			return nil, fmt.Errorf("failed to decode base64 firebase credentials from FCM_SERVICE_ACCOUNT_JSON: %v", err)
		}
		opt = option.WithCredentialsJSON(decoded)
		log.Println("FCM Service: Initializing from FCM_SERVICE_ACCOUNT_JSON environment variable.")
	} else {
		// Fallback to local file if environment variable is not set
		if _, err := os.Stat(localFilePath); os.IsNotExist(err) {
			return nil, fmt.Errorf("local firebase file not found: %s, and FCM_SERVICE_ACCOUNT_JSON environment variable is not set", localFilePath)
		}
		opt = option.WithCredentialsFile(localFilePath)
		log.Printf("FCM Service: Initializing from local file: %s.", localFilePath)
	}

	app, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		return nil, fmt.Errorf("error initializing firebase app: %v", err)
	}

	client, err := app.Messaging(context.Background())
	if err != nil {
		return nil, fmt.Errorf("error getting messaging client: %v", err)
	}

	return &FCMService{client: client}, nil
}

// SendPush UPDATED to fix 404 Error
func (s *FCMService) SendPush(ctx context.Context, tokens []notification.DeviceToken, title, body string, data map[string]any) error {
	if len(tokens) == 0 {
		return nil
	}

	// 1. Filter Android tokens
	var androidTokens []string
	for _, t := range tokens {
		if t.Platform == "android" || t.Platform == "" {
			androidTokens = append(androidTokens, t.Token)
		}
	}

	if len(androidTokens) == 0 {
		return nil
	}

	// 2. Convert data to map[string]string
	stringData := make(map[string]string)
	for k, v := range data {
		stringData[k] = fmt.Sprintf("%v", v)
	}

	// 3. SEND ONE BY ONE (Fixes the /batch 404 error)
	successCount := 0
	failureCount := 0

	for _, token := range androidTokens {
		message := &messaging.Message{
			Token: token,
			Notification: &messaging.Notification{
				Title: title,
				Body:  body,
			},
			Data: stringData,
			Android: &messaging.AndroidConfig{
				Priority: "high",
				Notification: &messaging.AndroidNotification{
					Sound: "default",
                    // Icon: "notification_icon", // Uncomment if you added the icon in app.json
				},
			},
		}

		// Send individually
		_, err := s.client.Send(ctx, message)
		if err != nil {
			log.Printf("FCM: Failed to send to token %s: %v", token, err)
			failureCount++
		} else {
			successCount++
		}
	}

	log.Printf("FCM: Sent %d messages, %d failed", successCount, failureCount)
	
    // If at least one succeeded, we consider it a success for the batch job
    // If all failed, return error
    if successCount == 0 && failureCount > 0 {
        return fmt.Errorf("all push notifications failed")
    }

	return nil
}