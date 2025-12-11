package skinport

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/andybalholm/brotli"
	"golang.org/x/sync/errgroup"
)

type Config struct {
	APIURL   string
	ClientID string
	APIKey   string
}

type cachedResponse struct {
	items  []ResponseItem
	expiry time.Time
}

type Client struct {
	client *http.Client
	config Config

	cacheMu   sync.RWMutex
	cacheData map[string]cachedResponse
}

func NewClient(cfg Config) *Client {
	return &Client{
		client: &http.Client{
			Transport: &AuthTransport{
				ClientID: cfg.ClientID,
				APIKey:   cfg.APIKey,
				Base:     http.DefaultTransport,
			},
			Timeout: 10 * time.Second,
		},
		config:    cfg,
		cacheData: make(map[string]cachedResponse),
	}
}

// AuthTransport adds Basic Auth headers
type AuthTransport struct {
	ClientID string
	APIKey   string
	Base     http.RoundTripper
}

func (t *AuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	auth := t.ClientID + ":" + t.APIKey
	encodedAuth := base64.StdEncoding.EncodeToString([]byte(auth))
	req.Header.Set("Authorization", "Basic "+encodedAuth)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Encoding", "br")
	return t.Base.RoundTrip(req)
}

func (c *Client) GetAllItems(ctx context.Context, appID, currency string) ([]ResponseItem, error) {
	// Default values if empty
	if appID == "" {
		appID = "730" // Default CS2
	}
	if currency == "" {
		currency = "EUR" // Default EUR
	}

	cacheKey := fmt.Sprintf("%s:%s", appID, currency)

	c.cacheMu.RLock()
	data, ok := c.cacheData[cacheKey]
	if ok && time.Now().Before(data.expiry) {
		c.cacheMu.RUnlock()
		return data.items, nil
	}
	c.cacheMu.RUnlock()

	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()

	// Double check logic
	data, ok = c.cacheData[cacheKey]
	if ok && time.Now().Before(data.expiry) {
		return data.items, nil
	}

	g, ctx := errgroup.WithContext(ctx)
	var tradableItems, nonTradableItems []RawItem

	// Request A: Tradable
	g.Go(func() error {
		var err error
		tradableItems, err = c.fetchItems(ctx, appID, currency, true)
		if err != nil {
			return fmt.Errorf("failed to fetch tradable items: %w", err)
		}
		return nil
	})

	// Request B: Non-Tradable
	g.Go(func() error {
		var err error
		nonTradableItems, err = c.fetchItems(ctx, appID, currency, false)
		if err != nil {
			return fmt.Errorf("failed to fetch non-tradable items: %w", err)
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Merge Logic
	itemMap := make(map[string]*ResponseItem)

	// Process tradable items
	for _, item := range tradableItems {
		itemMap[item.MarketHashName] = &ResponseItem{
			MarketHashName:   item.MarketHashName,
			Currency:         item.Currency,
			Slug:             item.Slug,
			MinPriceTradable: item.MinPrice,
			Quantity:         item.Quantity,
		}
	}

	// Process non-tradable items
	for _, item := range nonTradableItems {
		if existing, exists := itemMap[item.MarketHashName]; exists {
			existing.MinPriceNonTradable = item.MinPrice
			// Update quantity if needed, strictly speaking we might want to sum them
			existing.Quantity += item.Quantity
		} else {
			itemMap[item.MarketHashName] = &ResponseItem{
				MarketHashName:      item.MarketHashName,
				Currency:            item.Currency,
				Slug:                item.Slug,
				MinPriceNonTradable: item.MinPrice,
				Quantity:            item.Quantity,
			}
		}
	}

	var result []ResponseItem
	for _, item := range itemMap {
		result = append(result, *item)
	}

	// Update Cache
	c.cacheData[cacheKey] = cachedResponse{
		items:  result,
		expiry: time.Now().Add(5 * time.Minute),
	}

	return result, nil
}

func (c *Client) fetchItems(ctx context.Context, appID, currency string, tradable bool) ([]RawItem, error) {
	url := fmt.Sprintf("%s/items", c.config.APIURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Add("app_id", appID)
	q.Add("currency", currency)
	q.Add("tradable", fmt.Sprintf("%t", tradable))

	req.URL.RawQuery = q.Encode()

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.Header.Get("Content-Encoding") == "br" {
		resp.Body = &readCloserWrapper{Reader: brotli.NewReader(resp.Body), Closer: resp.Body}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var apiErr ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err == nil && len(apiErr.Errors) > 0 {
			return nil, &apiErr
		}
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var items []RawItem
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, err
	}

	return items, nil
}

type readCloserWrapper struct {
	io.Reader
	io.Closer
}

func (r *readCloserWrapper) Read(p []byte) (n int, err error) {
	return r.Reader.Read(p)
}

func (r *readCloserWrapper) Close() error {
	return r.Closer.Close()
}
