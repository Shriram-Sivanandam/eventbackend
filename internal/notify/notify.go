package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"golang.org/x/oauth2/google"
)

type NotificationType string

const (
	NotifNewRegistration      NotificationType = "new_registration"
	NotifRegistrationAccepted NotificationType = "registration_accepted"
	NotifRegistrationRejected NotificationType = "registration_rejected"
	NotifEventCancelled       NotificationType = "event_cancelled"
	NotifEventReminder        NotificationType = "event_reminder"
	NotifAttendeeLeft         NotificationType = "attendee_left"
	NotifRatingPrompt         NotificationType = "rating_prompt"
)

type Notification struct {
	Type    NotificationType
	ToToken string
	Title   string
	Body    string
	Data    map[string]string
}

func getAccessToken(ctx context.Context) (string, error) {
	credFile := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	if credFile == "" {
		return "", fmt.Errorf("GOOGLE_APPLICATION_CREDENTIALS not set")
	}

	data, err := os.ReadFile(credFile)
	if err != nil {
		return "", fmt.Errorf("read credentials file: %w", err)
	}

	conf, err := google.CredentialsFromJSONWithType(ctx, data,
		"https://www.googleapis.com/auth/firebase.messaging",
	)
	if err != nil {
		return "", fmt.Errorf("parse credentials: %w", err)
	}

	token, err := conf.TokenSource.Token()
	if err != nil {
		return "", fmt.Errorf("get token: %w", err)
	}

	return token.AccessToken, nil
}

func Send(ctx context.Context, n Notification) error {
	projectID := os.Getenv("FCM_PROJECT_ID")
	if projectID == "" {
		return fmt.Errorf("FCM_PROJECT_ID not set")
	}
	if n.ToToken == "" {
		return nil 
	}

	accessToken, err := getAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("get fcm access token: %w", err)
	}

	payload := map[string]any{
		"message": map[string]any{
			"token": n.ToToken,
			"notification": map[string]string{
				"title": n.Title,
				"body":  n.Body,
			},
			"data": n.Data,
			"android": map[string]any{
				"notification": map[string]string{
					"channel_id":             "spotlight_default",
					"notification_priority":  "PRIORITY_HIGH",
					"sound":                  "default",
				},
			},
			"apns": map[string]any{
				"payload": map[string]any{
					"aps": map[string]string{
						"sound": "default",
					},
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal fcm payload: %w", err)
	}

	url := fmt.Sprintf(
		"https://fcm.googleapis.com/v1/projects/%s/messages:send",
		projectID,
	)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create fcm request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("fcm send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fcm returned %d", resp.StatusCode)
	}

	return nil
}

func NewRegistration(hostToken, attendeeName, eventTitle, eventID string) Notification {
	return Notification{
		Type: NotifNewRegistration, ToToken: hostToken,
		Title: "New registration 🎟️",
		Body:  fmt.Sprintf("%s wants to join %s", attendeeName, eventTitle),
		Data:  map[string]string{"type": string(NotifNewRegistration), "event_id": eventID, "screen": "EventDashboard"},
	}
}

func RegistrationAccepted(attendeeToken, eventTitle, eventID string) Notification {
	return Notification{
		Type: NotifRegistrationAccepted, ToToken: attendeeToken,
		Title: "You're in! 🎉",
		Body:  fmt.Sprintf("Your spot at %s is confirmed.", eventTitle),
		Data:  map[string]string{"type": string(NotifRegistrationAccepted), "event_id": eventID, "screen": "EventDetails"},
	}
}

func RegistrationRejected(attendeeToken, eventTitle string) Notification {
	return Notification{
		Type: NotifRegistrationRejected, ToToken: attendeeToken,
		Title: "Registration update",
		Body:  fmt.Sprintf("Your registration for %s was not accepted.", eventTitle),
		Data:  map[string]string{"type": string(NotifRegistrationRejected), "screen": "RegisteredEvents"},
	}
}

func EventCancelled(attendeeToken, eventTitle string) Notification {
	return Notification{
		Type: NotifEventCancelled, ToToken: attendeeToken,
		Title: "Event cancelled",
		Body:  fmt.Sprintf("%s has been cancelled.", eventTitle),
		Data:  map[string]string{"type": string(NotifEventCancelled), "screen": "RegisteredEvents"},
	}
}

func EventReminder(attendeeToken, eventTitle, eventID string) Notification {
	return Notification{
		Type: NotifEventReminder, ToToken: attendeeToken,
		Title: "Starting soon ⏰",
		Body:  fmt.Sprintf("%s starts in 1 hour!", eventTitle),
		Data:  map[string]string{"type": string(NotifEventReminder), "event_id": eventID, "screen": "EventDetails"},
	}
}

func AttendeeLeft(hostToken, attendeeName, eventTitle, eventID string) Notification {
	return Notification{
		Type: NotifAttendeeLeft, ToToken: hostToken,
		Title: "Spot opened up",
		Body:  fmt.Sprintf("%s cancelled their spot at %s.", attendeeName, eventTitle),
		Data:  map[string]string{"type": string(NotifAttendeeLeft), "event_id": eventID, "screen": "EventDashboard"},
	}
}

func RatingPrompt(attendeeToken, eventTitle, eventID string) Notification {
	return Notification{
		Type: NotifRatingPrompt, ToToken: attendeeToken,
		Title: "How was it? ⭐",
		Body:  fmt.Sprintf("Rate your experience at %s.", eventTitle),
		Data:  map[string]string{"type": string(NotifRatingPrompt), "event_id": eventID, "screen": "RegisteredEvents"},
	}
}