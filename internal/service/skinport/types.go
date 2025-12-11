package skinport

import (
	"fmt"
)

type RawItem struct {
	MarketHashName string   `json:"market_hash_name"`
	Currency       string   `json:"currency"`
	Slug           string   `json:"slug"`
	MinPrice       *float64 `json:"min_price"`
	Quantity       int      `json:"quantity"`
}

type ResponseItem struct {
	MarketHashName      string   `json:"market_hash_name"`
	Currency            string   `json:"currency"`
	Slug                string   `json:"slug"`
	MinPriceTradable    *float64 `json:"min_price_tradable"`
	MinPriceNonTradable *float64 `json:"min_price_non_tradable"`
	Quantity            int      `json:"quantity"`
}

type APIError struct {
	ID      string `json:"id"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	Errors []APIError `json:"errors"`
}

func (e *ErrorResponse) Error() string {
	return fmt.Sprintf("skinport api error: %v", e.Errors)
}
