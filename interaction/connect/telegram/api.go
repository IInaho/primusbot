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
	"strings"
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

// messageBody builds the common sendMessage/editMessageText payload. Plain
// mode omits parse_mode (fallback for HTML parse failures).
func messageBody(chatID int64, text string, plain bool) map[string]any {
	body := map[string]any{
		"chat_id":                  chatID,
		"text":                     text,
		"disable_web_page_preview": true,
	}
	if !plain {
		body["parse_mode"] = "HTML"
	}
	return body
}

// isParseFailure reports whether err is a Telegram HTML parse rejection
// (HTTP 400 "can't parse entities"), i.e. worth retrying as plain text.
func isParseFailure(err error) bool {
	if err == nil {
		return false
	}
	if httpErr, ok := err.(*sharedhttp.HTTPError); ok {
		return httpErr.StatusCode == 400
	}
	return strings.Contains(strings.ToLower(err.Error()), "can't parse")
}

// sendHTML posts text with HTML parse mode, falling back to plain text when
// Telegram rejects the markup — a message with literal tags beats a lost one.
func (c *apiClient) sendHTML(ctx context.Context, endpoint string, body map[string]any) (json.RawMessage, error) {
	result, err := requestJSON[json.RawMessage](ctx, c, http.MethodPost, endpoint, nil, body)
	if err == nil || !isParseFailure(err) {
		return result, err
	}
	body["parse_mode"] = nil
	delete(body, "parse_mode")
	return requestJSON[json.RawMessage](ctx, c, http.MethodPost, endpoint, nil, body)
}

func (c *apiClient) sendMessageHTML(ctx context.Context, chatID int64, text string) error {
	_, err := c.sendHTML(ctx, "sendMessage", messageBody(chatID, text, false))
	return err
}

// sendMessageID sends a message and returns its ID (for later edits).
func (c *apiClient) sendMessageID(ctx context.Context, chatID int64, text string) (int, error) {
	result, err := c.sendHTML(ctx, "sendMessage", messageBody(chatID, text, false))
	if err != nil {
		return 0, err
	}
	var msg Message
	if err := json.Unmarshal(result, &msg); err != nil {
		return 0, err
	}
	return msg.MessageID, nil
}

// editMessageText rewrites a previously sent message in place (used by the
// streaming preview). Same HTML-with-plain-fallback behavior as sends.
func (c *apiClient) editMessageText(ctx context.Context, chatID int64, messageID int, text string) error {
	body := messageBody(chatID, text, false)
	body["message_id"] = messageID
	_, err := c.sendHTML(ctx, "editMessageText", body)
	return err
}

type inlineKeyboardMarkup struct {
	InlineKeyboard [][]inlineKeyboardButton `json:"inline_keyboard"`
}

type inlineKeyboardButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data"`
}

// sendMessageWithKeyboard sends a message with an inline keyboard and
// returns its message ID (needed to edit/strip the keyboard later).
func (c *apiClient) sendMessageWithKeyboard(ctx context.Context, chatID int64, text string, keyboard inlineKeyboardMarkup) (int, error) {
	body := messageBody(chatID, text, false)
	body["reply_markup"] = keyboard
	result, err := c.sendHTML(ctx, "sendMessage", body)
	if err != nil {
		return 0, err
	}
	var msg Message
	if err := json.Unmarshal(result, &msg); err != nil {
		return 0, err
	}
	return msg.MessageID, nil
}

// emptyKeyboard removes all buttons from a message when used as the reply
// markup of an edit.
var emptyKeyboard = inlineKeyboardMarkup{InlineKeyboard: [][]inlineKeyboardButton{}}

// editMessage rewrites a message's text and, when markup is non-nil, its
// reply markup (pass &emptyKeyboard to strip buttons). The Telegram
// "message is not modified" rejection is swallowed — repeated
// terminalization is idempotent by design.
func (c *apiClient) editMessage(ctx context.Context, chatID int64, messageID int, text string, markup *inlineKeyboardMarkup) error {
	body := messageBody(chatID, text, false)
	body["message_id"] = messageID
	if markup != nil {
		body["reply_markup"] = *markup
	}
	_, err := c.sendHTML(ctx, "editMessageText", body)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "not modified") {
		return nil
	}
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
