package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"time"
)

type TelegramAdapter struct {
	token        string
	allowedUsers []int64
	client       *http.Client
	baseURL      string
}

func NewTelegram(tokenEnv string, allowedUsers []int64) (*TelegramAdapter, error) {
	token := os.Getenv(tokenEnv)
	if token == "" {
		return nil, fmt.Errorf("environment variable %s is not set", tokenEnv)
	}
	return &TelegramAdapter{
		token:        token,
		allowedUsers: allowedUsers,
		client:       &http.Client{Timeout: 60 * time.Second},
		baseURL:      "https://api.telegram.org/bot" + token,
	}, nil
}

func (t *TelegramAdapter) Name() string { return "telegram" }

func (t *TelegramAdapter) Start(ctx context.Context, handler Handler) error {
	slog.Info("telegram adapter started")

	// Verify bot token
	me, err := t.getMe()
	if err != nil {
		return fmt.Errorf("telegram auth failed: %w", err)
	}
	slog.Info("telegram bot connected", "username", me)

	var offset int64
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		updates, err := t.getUpdates(ctx, offset)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			slog.Warn("telegram getUpdates failed", "error", err)
			time.Sleep(3 * time.Second)
			continue
		}

		for _, update := range updates {
			offset = update.UpdateID + 1

			if update.Message == nil || update.Message.Text == "" {
				continue
			}

			userID := update.Message.From.ID
			chatID := update.Message.Chat.ID

			// Check allowed users
			if len(t.allowedUsers) > 0 && !slices.Contains(t.allowedUsers, userID) {
				slog.Warn("telegram unauthorized user", "user_id", userID)
				continue
			}

			slog.Info("telegram message received",
				"user_id", userID,
				"text", update.Message.Text,
			)

			// Process message
			go t.handleMessage(ctx, handler, chatID, update.Message.Text)
		}
	}
}

func (t *TelegramAdapter) Send(ctx context.Context, message string) error {
	for _, userID := range t.allowedUsers {
		if err := t.sendMessage(userID, message); err != nil {
			slog.Error("telegram send failed", "user_id", userID, "error", err)
		}
	}
	return nil
}

func (t *TelegramAdapter) Stop() error {
	slog.Info("telegram adapter stopped")
	return nil
}

func (t *TelegramAdapter) handleMessage(ctx context.Context, handler Handler, chatID int64, text string) {
	// Send typing indicator
	t.sendChatAction(chatID, "typing")

	response, err := handler(ctx, text)
	if err != nil {
		slog.Error("telegram handler error", "error", err)
		response = fmt.Sprintf("Error: %v", err)
	}

	if response == "" {
		response = "(no response)"
	}

	// Telegram has a 4096 character limit per message
	for len(response) > 0 {
		chunk := response
		if len(chunk) > 4096 {
			chunk = chunk[:4096]
			response = response[4096:]
		} else {
			response = ""
		}

		if err := t.sendMessage(chatID, chunk); err != nil {
			slog.Error("telegram sendMessage failed", "error", err)
		}
	}
}

// Telegram Bot API types

type tgUpdate struct {
	UpdateID int64      `json:"update_id"`
	Message  *tgMessage `json:"message"`
}

type tgMessage struct {
	Chat *tgChat `json:"chat"`
	From *tgUser `json:"from"`
	Text string  `json:"text"`
}

type tgChat struct {
	ID int64 `json:"id"`
}

type tgUser struct {
	ID int64 `json:"id"`
}

// Telegram Bot API methods

func (t *TelegramAdapter) getMe() (string, error) {
	resp, err := t.apiCall("getMe", nil)
	if err != nil {
		return "", err
	}
	var result struct {
		OK     bool `json:"ok"`
		Result struct {
			Username string `json:"username"`
		} `json:"result"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", err
	}
	if !result.OK {
		return "", fmt.Errorf("getMe returned ok=false")
	}
	return result.Result.Username, nil
}

func (t *TelegramAdapter) getUpdates(ctx context.Context, offset int64) ([]tgUpdate, error) {
	body := map[string]any{
		"offset":  offset,
		"timeout": 30,
	}
	resp, err := t.apiCallWithContext(ctx, "getUpdates", body)
	if err != nil {
		return nil, err
	}
	var result struct {
		OK     bool       `json:"ok"`
		Result []tgUpdate `json:"result"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}
	if !result.OK {
		return nil, fmt.Errorf("getUpdates returned ok=false")
	}
	return result.Result, nil
}

func (t *TelegramAdapter) sendMessage(chatID int64, text string) error {
	_, err := t.apiCall("sendMessage", map[string]any{
		"chat_id": chatID,
		"text":    text,
	})
	return err
}

func (t *TelegramAdapter) sendChatAction(chatID int64, action string) {
	t.apiCall("sendChatAction", map[string]any{
		"chat_id": chatID,
		"action":  action,
	})
}

func (t *TelegramAdapter) apiCall(method string, body map[string]any) ([]byte, error) {
	return t.apiCallWithContext(context.Background(), method, body)
}

func (t *TelegramAdapter) apiCallWithContext(ctx context.Context, method string, body map[string]any) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.baseURL+"/"+method, reqBody)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}
