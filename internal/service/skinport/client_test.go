package skinport

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func floatPtr(v float64) *float64 {
	return &v
}

func TestGetAllItems_Success(t *testing.T) {
	// 1. Setup Mock Server
	// We expect 2 calls: one for tradable=true, one for tradable=false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Basic Auth check
		auth := r.Header.Get("Authorization")
		expectedAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("client_id:api_key"))
		if auth != expectedAuth {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		tradable := r.URL.Query().Get("tradable")
		w.Header().Set("Content-Type", "application/json")

		if tradable == "true" {
			// Return tradable items
			json.NewEncoder(w).Encode([]RawItem{
				{MarketHashName: "Item A", Currency: "EUR", Slug: "item-a", MinPrice: floatPtr(10.5), Quantity: 5},
				{MarketHashName: "Item B", Currency: "EUR", Slug: "item-b", MinPrice: floatPtr(20.0), Quantity: 1},
			})
		} else {
			// Return non-tradable items
			json.NewEncoder(w).Encode([]RawItem{
				{MarketHashName: "Item A", Currency: "EUR", Slug: "item-a", MinPrice: floatPtr(9.0), Quantity: 2},  // Cheaper, exists in tradable
				{MarketHashName: "Item C", Currency: "EUR", Slug: "item-c", MinPrice: floatPtr(30.0), Quantity: 3}, // Only non-tradable
			})
		}
	}))
	defer ts.Close()

	// 2. Setup Client
	cfg := Config{
		APIURL:   ts.URL,
		ClientID: "client_id",
		APIKey:   "api_key",
	}
	client := NewClient(cfg)

	// 3. Execute
	items, err := client.GetAllItems(context.Background(), "730", "EUR")

	// 4. Verify
	assert.NoError(t, err)
	assert.Len(t, items, 3) // Item A, Item B, Item C

	// Convert to map for easier checking
	itemMap := make(map[string]ResponseItem)
	for _, item := range items {
		itemMap[item.MarketHashName] = item
	}

	// Check Item A (Exists in both)
	itemA, ok := itemMap["Item A"]
	assert.True(t, ok)
	if assert.NotNil(t, itemA.MinPriceTradable) {
		assert.Equal(t, 10.5, *itemA.MinPriceTradable)
	}
	if assert.NotNil(t, itemA.MinPriceNonTradable) {
		assert.Equal(t, 9.0, *itemA.MinPriceNonTradable)
	}
	assert.Equal(t, 7, itemA.Quantity) // 5 + 2

	// Check Item B (Only tradable)
	itemB, ok := itemMap["Item B"]
	assert.True(t, ok)
	if assert.NotNil(t, itemB.MinPriceTradable) {
		assert.Equal(t, 20.0, *itemB.MinPriceTradable)
	}
	assert.Nil(t, itemB.MinPriceNonTradable) // Default is nil if not set? No, struct default is nil. Logic says default 0?
	// The client logic does not set it to 0 explicitly if missing, it's a pointer.
	// So it should be nil.
	assert.Equal(t, 1, itemB.Quantity)

	// Check Item C (Only non-tradable)
	itemC, ok := itemMap["Item C"]
	assert.True(t, ok)
	assert.Nil(t, itemC.MinPriceTradable)
	if assert.NotNil(t, itemC.MinPriceNonTradable) {
		assert.Equal(t, 30.0, *itemC.MinPriceNonTradable)
	}
	assert.Equal(t, 3, itemC.Quantity)
}

func TestGetAllItems_Cache(t *testing.T) {
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		json.NewEncoder(w).Encode([]RawItem{})
	}))
	defer ts.Close()

	client := NewClient(Config{APIURL: ts.URL})

	// First call - should hit server (2 requests: tradable T/F)
	_, err := client.GetAllItems(context.Background(), "", "")
	assert.NoError(t, err)
	assert.Equal(t, 2, requestCount)

	// Second call - should hit cache
	_, err = client.GetAllItems(context.Background(), "", "")
	assert.NoError(t, err)
	assert.Equal(t, 2, requestCount, "Should not increment request count due to caching")
}

func TestGetAllItems_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"errors":[{"id":"server_error","message":"Something went wrong"}]}`))
	}))
	defer ts.Close()

	client := NewClient(Config{APIURL: ts.URL})

	_, err := client.GetAllItems(context.Background(), "", "")
	assert.Error(t, err)
	// Error could be from tradable or non-tradable fetching
	assert.Contains(t, err.Error(), "failed to fetch")
}

func TestGetAllItems_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`invalid-json`))
	}))
	defer ts.Close()

	client := NewClient(Config{APIURL: ts.URL})

	_, err := client.GetAllItems(context.Background(), "", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid character")
}

func TestGetAllItems_LargeDataset(t *testing.T) {
	// Generate 1000000 items
	count := 1000000

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Stream the response manually to avoid huge memory allocation in mock
		w.Write([]byte("["))
		for i := 0; i < count; i++ {
			if i > 0 {
				w.Write([]byte(","))
			}
			// Use simple string concat or fmt for speed in test, mock data
			// Item-0, Item-1, ...
			// If tradable: Item-0...Item-49999
			// If non-tradable: Item-0...Item-49999 (same names to force merge)

			itemJSON := fmt.Sprintf(`{
				"market_hash_name": "Item-%d",
				"currency": "EUR",
				"suggested_price": 13.18,
				"item_page": "https://skinport.com/item/csgo/item-%d",
				"market_page": "https://skinport.com/market/730?cat=Rifle&item=Item-%d",
				"min_price": %d.5,
				"max_price": 18.22,
				"mean_price": 12.58,
				"median_price": 13.37,
				"quantity": 1,
				"created_at": 1535988253,
				"updated_at": 1568073728,
				"slug": "item-%d"
			}`, i, i, i, i, i)
			w.Write([]byte(itemJSON))
		}
		w.Write([]byte("]"))
	}))
	defer ts.Close()

	client := NewClient(Config{APIURL: ts.URL})

	start := time.Now()
	items, err := client.GetAllItems(context.Background(), "730", "EUR")
	duration := time.Since(start)

	assert.NoError(t, err)
	assert.Len(t, items, count, "Should have %d merged items", count)

	// Sanity check a few items
	itemMap := make(map[string]ResponseItem)
	for _, item := range items {
		itemMap[item.MarketHashName] = item
	}

	// Check first item
	item0, ok := itemMap["Item-0"]
	assert.True(t, ok)
	assert.Equal(t, 2, item0.Quantity) // 1 tradable + 1 non-tradable
	if assert.NotNil(t, item0.MinPriceTradable) {
		assert.Equal(t, 0.5, *item0.MinPriceTradable)
	}
	if assert.NotNil(t, item0.MinPriceNonTradable) {
		assert.Equal(t, 0.5, *item0.MinPriceNonTradable)
	}

	// Check last item
	itemLast, ok := itemMap[fmt.Sprintf("Item-%d", count-1)]
	assert.True(t, ok)
	assert.Equal(t, 2, itemLast.Quantity)

	t.Logf("Processed %d items in %v", count*2, duration)
}
