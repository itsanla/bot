package activecollab

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client

	userCacheMu sync.RWMutex
	userCache   map[int64]string
}

func NewClient(baseURL, token string) *Client {
	baseURL = strings.TrimRight(baseURL, "/")
	return &Client{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
		userCache: make(map[int64]string),
	}
}

func (c *Client) GetNotifications(ctx context.Context) (*NotificationResponse, error) {
	url := fmt.Sprintf("%s/api/v1/notifications", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Angie-AuthApiToken", c.token)
	// Critical: Cloudflare WAF on collab.javan.co.id blocks default Go/Python User-Agents
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to activecollab failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read activecollab response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("activecollab API returned HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var notifResp NotificationResponse
	if err := json.Unmarshal(bodyBytes, &notifResp); err != nil {
		return nil, fmt.Errorf("failed to parse activecollab json response: %w", err)
	}

	return &notifResp, nil
}

// GetUserName resolves a user ID to a display name, caching results in memory.
func (c *Client) GetUserName(ctx context.Context, userID int64) string {
	c.userCacheMu.RLock()
	name, ok := c.userCache[userID]
	c.userCacheMu.RUnlock()
	if ok && name != "" {
		return name
	}

	// Fetch users list to populate cache if empty
	c.refreshUserCache(ctx)

	c.userCacheMu.RLock()
	defer c.userCacheMu.RUnlock()
	if n, found := c.userCache[userID]; found && n != "" {
		return n
	}
	return fmt.Sprintf("User #%d", userID)
}

func (c *Client) refreshUserCache(ctx context.Context) {
	url := fmt.Sprintf("%s/api/v1/users", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return
	}
	req.Header.Set("X-Angie-AuthApiToken", c.token)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64)")

	resp, err := c.httpClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return
	}
	defer resp.Body.Close()

	var users []User
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return
	}

	c.userCacheMu.Lock()
	defer c.userCacheMu.Unlock()
	for _, u := range users {
		name := u.DisplayName
		if name == "" {
			name = u.Name
		}
		c.userCache[u.ID] = name
	}
}
