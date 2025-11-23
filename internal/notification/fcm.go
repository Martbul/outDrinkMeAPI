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

// Initialize connection to Firebase
func NewFCMService(localFilePath string) (*FCMService, error) {
	var opt option.ClientOption

	// 1. Check for Production Environment Variable first
	encodedCreds := os.Getenv("FIREBASE_CREDENTIALS_BASE64")
	
	if encodedCreds != "" {
		log.Println("FCM: Loading credentials from Environment Variable")
		decoded, err := base64.StdEncoding.DecodeString(encodedCreds)
		if err != nil {
			return nil, fmt.Errorf("failed to decode base64 firebase credentials: %v", err)
		}
		opt = option.WithCredentialsJSON(decoded)
	} else {
		// 2. Fallback to Local File (for Development)
		log.Println("FCM: Loading credentials from local file")
		if _, err := os.Stat(localFilePath); os.IsNotExist(err) {
			return nil, fmt.Errorf("local firebase file not found: %s", localFilePath)
		}
		opt = option.WithCredentialsFile(localFilePath)
	}

	// Initialize App
	app, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		return nil, fmt.Errorf("error initializing firebase app: %v", err)
	}

	// Get Messaging Client
	client, err := app.Messaging(context.Background())
	if err != nil {
		return nil, fmt.Errorf("error getting messaging client: %v", err)
	}

	return &FCMService{client: client}, nil
}

// SendPush sends a message to multiple Android devices
func (s *FCMService) SendPush(ctx context.Context, tokens []notification.DeviceToken, title, body string, data map[string]any) error {
	if len(tokens) == 0 {
		return nil
	}

	// 1. Extract token strings and filter for Android only (if you ever add iOS later)
	var androidTokens []string
	for _, t := range tokens {
		if t.Platform == "android" || t.Platform == "" { // Default to android if unspecified
			androidTokens = append(androidTokens, t.Token)
		}
	}

	if len(androidTokens) == 0 {
		return nil
	}

	// 2. Convert data to map[string]string (FCM Requirement)
	stringData := make(map[string]string)
	for k, v := range data {
		stringData[k] = fmt.Sprintf("%v", v)
	}

	// 3. Construct the Message
	message := &messaging.MulticastMessage{
		Tokens: androidTokens,
		
		// The visible notification on the phone
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		
		// Background data for your app to handle when clicked
		Data: stringData,

		// Android Specifics
		Android: &messaging.AndroidConfig{
			Priority: "high", // Wakes up the phone
			Notification: &messaging.AndroidNotification{
				Sound: "default",
				// Icon: "ic_notification", // Ensure this resource exists in your Android App's drawable folder
				// ChannelId: "general",    // Important for Android 8.0+ (Oreo)
			},
		},
	}

	// 4. Send
	br, err := s.client.SendMulticast(ctx, message)
	if err != nil {
		return err
	}

	// 5. Log results (In production, remove invalid tokens here)
	if br.FailureCount > 0 {
		log.Printf("FCM: Sent %d messages, %d failed", br.SuccessCount, br.FailureCount)
	} else {
		log.Printf("FCM: Successfully sent to %d devices", br.SuccessCount)
	}

	return nil
}