package sender

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type notificationRequest struct {
	GroupChatID  string       `json:"group_chat_id"`
	Notification notification `json:"notification"`
}

type notification struct {
	Status string `json:"status"`
	Body   string `json:"body"`
}

type sendResponse struct {
	Status string `json:"status"`
}

// Send posts a notification message to a chat via BotX API.
// Endpoint: POST /api/v4/botx/notifications/direct
func Send(ctx context.Context, host, token, chatID, message string) error {
	return sendWithClient(ctx, fmt.Sprintf("https://%s", host), token, chatID, message, &http.Client{Timeout: 30 * time.Second})
}

func sendWithClient(ctx context.Context, baseURL, token, chatID, message string, client *http.Client) error {
	payload := notificationRequest{
		GroupChatID: chatID,
		Notification: notification{
			Status: "ok",
			Body:   message,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling request: %w", err)
	}

	url := baseURL + "/api/v4/botx/notifications/direct"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("sending notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("notification failed: HTTP %d", resp.StatusCode)
	}

	if resp.StatusCode == http.StatusAccepted {
		return nil
	}

	var result sendResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}

	if result.Status != "ok" {
		return fmt.Errorf("unexpected status: %s", result.Status)
	}

	return nil
}

// ErrUnauthorized indicates the token is invalid or expired.
var ErrUnauthorized = fmt.Errorf("unauthorized (HTTP 401)")
