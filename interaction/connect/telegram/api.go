package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	sharedhttp "nekocode/util/http"
)

const defaultAPIBase = "https://api.telegram.org"

type apiClient struct {
	token  string
	base   string
	client *http.Client
}

func newAPIClient(token string) *apiClient {
	return &apiClient{
		token: token,
		base:  defaultAPIBase,
		client: &http.Client{
			Timeout:   40 * time.Second,
			Transport: sharedhttp.SharedTransport,
		},
	}
}

type apiResponse[T any] struct {
	OK          bool   `json:"ok"`
	Description string `json:"description,omitempty"`
	Result      T      `json:"result"`
}

type User struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
}

type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

type Message struct {
	MessageID int    `json:"message_id"`
	From      *User  `json:"from"`
	Chat      Chat   `json:"chat"`
	Text      string `json:"text"`
}

type CallbackQuery struct {
	ID      string   `json:"id"`
	From    User     `json:"from"`
	Message *Message `json:"message"`
	Data    string   `json:"data"`
}

type Update struct {
	UpdateID      int            `json:"update_id"`
	Message       *Message       `json:"message"`
	CallbackQuery *CallbackQuery `json:"callback_query"`
}

func (c *apiClient) getMe(ctx context.Context) (User, error) {
	return requestJSON[User](ctx, c, http.MethodGet, "getMe", nil, nil)
}

func (c *apiClient) getUpdates(ctx context.Context, offset int, timeoutSeconds int) ([]Update, error) {
	values := url.Values{}
	if offset > 0 {
		values.Set("offset", strconv.Itoa(offset))
	}
	values.Set("timeout", strconv.Itoa(timeoutSeconds))
	return requestJSON[[]Update](ctx, c, http.MethodGet, "getUpdates", values, nil)
}

func (c *apiClient) sendMessage(ctx context.Context, chatID int64, text string) error {
	return c.sendMessageHTML(ctx, chatID, text)
}

func (c *apiClient) sendMessageHTML(ctx context.Context, chatID int64, text string) error {
	body := map[string]any{
		"chat_id":                  chatID,
		"text":                     text,
		"parse_mode":               "HTML",
		"disable_web_page_preview": true,
	}
	_, err := requestJSON[json.RawMessage](ctx, c, http.MethodPost, "sendMessage", nil, body)
	return err
}

type inlineKeyboardMarkup struct {
	InlineKeyboard [][]inlineKeyboardButton `json:"inline_keyboard"`
}

type inlineKeyboardButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data"`
}

func (c *apiClient) sendMessageWithKeyboard(ctx context.Context, chatID int64, text string, keyboard inlineKeyboardMarkup) error {
	body := map[string]any{
		"chat_id":                  chatID,
		"text":                     text,
		"parse_mode":               "HTML",
		"disable_web_page_preview": true,
		"reply_markup":             keyboard,
	}
	_, err := requestJSON[json.RawMessage](ctx, c, http.MethodPost, "sendMessage", nil, body)
	return err
}

func (c *apiClient) answerCallbackQuery(ctx context.Context, callbackID, text string) error {
	body := map[string]any{"callback_query_id": callbackID}
	if text != "" {
		body["text"] = text
	}
	_, err := requestJSON[json.RawMessage](ctx, c, http.MethodPost, "answerCallbackQuery", nil, body)
	return err
}

func requestJSON[T any](ctx context.Context, c *apiClient, method, endpoint string, query url.Values, body any) (T, error) {
	var zero T
	u := fmt.Sprintf("%s/bot%s/%s", c.base, c.token, endpoint)
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return zero, err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return zero, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return zero, sharedhttp.NewHTTPError(resp.StatusCode, string(data))
	}
	var decoded apiResponse[T]
	if err := json.Unmarshal(data, &decoded); err != nil {
		return zero, err
	}
	if !decoded.OK {
		return zero, fmt.Errorf("telegram api: %s", decoded.Description)
	}
	return decoded.Result, nil
}
