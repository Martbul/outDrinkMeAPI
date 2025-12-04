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

// NewFCMService remains the same...
func NewFCMService(localFilePath string) (*FCMService, error) {
	// ... (Your existing initialization code keeps logic here) ...
    // Copy-paste your existing NewFCMService logic here, no changes needed
    var opt option.ClientOption
	encodedCreds := os.Getenv("FIREBASE_CREDENTIALS_BASE64")
	if encodedCreds != "" {
		decoded, err := base64.StdEncoding.DecodeString(encodedCreds)
		if err != nil {
			return nil, fmt.Errorf("failed to decode base64 firebase credentials: %v", err)
		}
		opt = option.WithCredentialsJSON(decoded)
	} else {
		if _, err := os.Stat(localFilePath); os.IsNotExist(err) {
			return nil, fmt.Errorf("local firebase file not found: %s", localFilePath)
		}
		opt = option.WithCredentialsFile(localFilePath)
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