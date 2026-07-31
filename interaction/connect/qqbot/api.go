package qqbot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	prodBaseURL    = "https://api.sgroup.qq.com"
	sandboxBaseURL = "https://sandbox.api.sgroup.qq.com"
	tokenURL       = "https://bots.qq.com/app/getAppAccessToken"
)

// apiClient 是 QQ 机器人开放平台的 REST 客户端：负责 access token 的
// 获取与自动刷新（约 80% 寿命时刷新）、gateway 地址查询和消息发送。
type apiClient struct {
	appID     string
	appSecret string
	sandbox   bool
	http      *http.Client

	mu        sync.Mutex
	token     string
	expiresAt time.Time
}

func newAPIClient(appID, appSecret string, sandbox bool) *apiClient {
	return &apiClient{
		appID:     appID,
		appSecret: appSecret,
		sandbox:   sandbox,
		http:      &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *apiClient) baseURL() string {
	if c.sandbox {
		return sandboxBaseURL
	}
	return prodBaseURL
}

// accessToken 返回可用 token；过期（达到约 80% 寿命）时自动重新获取。
func (c *apiClient) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Now().Before(c.expiresAt) {
		return c.token, nil
	}
	body, err := json.Marshal(map[string]string{"appId": c.appID, "clientSecret": c.appSecret})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("getAppAccessToken: status %d: %s", resp.StatusCode, truncateRunes(string(data), 200))
	}
	var out struct {
		AccessToken string      `json:"access_token"`
		ExpiresIn   json.Number `json:"expires_in"` // 平台返回的是字符串秒数
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", fmt.Errorf("getAppAccessToken: %w", err)
	}
	if out.AccessToken == "" {
		return "", fmt.Errorf("getAppAccessToken: empty access_token")
	}
	secs, err := out.ExpiresIn.Int64()
	if err != nil || secs <= 0 {
		secs = 7200
	}
	c.token = out.AccessToken
	c.expiresAt = time.Now().Add(time.Duration(secs*8/10) * time.Second)
	return c.token, nil
}

// doJSON 发送带 QQBot 鉴权头的 REST 请求。
func (c *apiClient) doJSON(ctx context.Context, method, url string, body, out any) error {
	token, err := c.accessToken(ctx)
	if err != nil {
		return err
	}
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "QQBot "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s: status %d: %s", method, url, resp.StatusCode, truncateRunes(string(data), 200))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(data, out)
}

// gatewayURL 查询 WebSocket gateway 地址。
func (c *apiClient) gatewayURL(ctx context.Context) (string, error) {
	var out struct {
		URL string `json:"url"`
	}
	if err := c.doJSON(ctx, http.MethodGet, c.baseURL()+"/gateway", nil, &out); err != nil {
		return "", err
	}
	if out.URL == "" {
		return "", fmt.Errorf("gateway response missing url")
	}
	return out.URL, nil
}

// messageBody 构造发消息请求体；msgID 非空时作为被动回复携带。
func messageBody(content, msgID string) map[string]any {
	body := map[string]any{"msg_type": 0, "content": content}
	if msgID != "" {
		body["msg_id"] = msgID
	}
	return body
}

func (c *apiClient) sendGroupMessage(ctx context.Context, groupOpenID, content, msgID string) error {
	url := fmt.Sprintf("%s/v2/groups/%s/messages", c.baseURL(), groupOpenID)
	return c.doJSON(ctx, http.MethodPost, url, messageBody(content, msgID), nil)
}

func (c *apiClient) sendC2CMessage(ctx context.Context, openid, content, msgID string) error {
	url := fmt.Sprintf("%s/v2/users/%s/messages", c.baseURL(), openid)
	return c.doJSON(ctx, http.MethodPost, url, messageBody(content, msgID), nil)
}
