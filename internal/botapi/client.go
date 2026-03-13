package botapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client is a BotX API client.
type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

// NewClient creates a Client for the given host and token.
func NewClient(host, token string) *Client {
	return &Client{
		BaseURL:    fmt.Sprintf("https://%s", host),
		Token:      token,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// ChatInfo holds information about a single chat.
type ChatInfo struct {
	GroupChatID   string   `json:"group_chat_id"`
	Name          string   `json:"name"`
	Description   *string  `json:"description"`
	ChatType      string   `json:"chat_type"`
	Members       []string `json:"members"`
	SharedHistory bool     `json:"shared_history"`
}

type listChatsResponse struct {
	Result []ChatInfo `json:"result"`
}

// ListChats returns all chats the bot is a member of.
func (c *Client) ListChats(ctx context.Context) ([]ChatInfo, error) {
	url := c.BaseURL + "/api/v3/botx/chats/list"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("listing chats: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list chats failed: HTTP %d", resp.StatusCode)
	}

	var result listChatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return result.Result, nil
}

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

// ErrUnauthorized indicates the token is invalid or expired.
var ErrUnauthorized = fmt.Errorf("unauthorized (HTTP 401)")

// SendNotification posts a message to a chat via BotX API.
func (c *Client) SendNotification(ctx context.Context, chatID, message string) error {
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

	url := c.BaseURL + "/api/v4/botx/notifications/direct"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}

	if resp.StatusCode == http.StatusAccepted {
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("notification failed: HTTP %d", resp.StatusCode)
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
