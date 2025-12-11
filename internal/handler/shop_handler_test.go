package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"fsanano/go-test/internal/handler"
	"fsanano/go-test/internal/repository"
	"fsanano/go-test/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func setupTestDB(t *testing.T) *pgxpool.Pool {
	_ = godotenv.Load("../../.env")

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Fatalf("DATABASE_URL not set")
	}

	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		t.Fatalf("Unable to parse database URL: %v", err)
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatalf("Unable to connect to database: %v", err)
	}

	// Wait for connection
	if err := pool.Ping(context.Background()); err != nil {
		t.Fatalf("Unable to ping database: %v", err)
	}

	// Truncate tables to ensure clean state
	tables := []string{"orders", "users", "items"} // Order matters due to FK
	for _, table := range tables {
		_, err := pool.Exec(context.Background(), fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", table))
		if err != nil {
			t.Fatalf("Failed to truncate table %s: %v", table, err)
		}
	}

	return pool
}

func TestBuyItem_Integration(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()

	// 1. Seed Data
	userID := 1
	initialBalance := 100.0
	_, err := pool.Exec(ctx, "INSERT INTO users (id, first_name, last_name, balance) VALUES ($1, 'Test', 'User', $2)", userID, initialBalance)
	if err != nil {
		t.Fatalf("Failed to seed user: %v", err)
	}

	itemID := 1
	itemPrice := 10.0
	initialStock := 5
	_, err = pool.Exec(ctx, "INSERT INTO items (id, name, price, stock) VALUES ($1, 'Test Item', $2, $3)", itemID, itemPrice, initialStock)
	if err != nil {
		t.Fatalf("Failed to seed item: %v", err)
	}

	// 2. Setup Handler
	repo := repository.NewShopRepository(pool)
	svc := service.NewShopService(repo)
	h := handler.NewShopHandler(svc)

	// 3. Perform Request (Success Case)
	buyQty := 1
	reqBody, _ := json.Marshal(map[string]interface{}{
		"user_id": userID,
		"item_id": itemID,
		"count":   buyQty,
	})

	req := httptest.NewRequest(http.MethodPost, "/buy", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.BuyItem(w, req)

	// 4. Verify Response
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 OK, got %d", resp.StatusCode)
	}

	// 5. Verify DB State
	var newBalance float64
	err = pool.QueryRow(ctx, "SELECT balance FROM users WHERE id = $1", userID).Scan(&newBalance)
	if err != nil {
		t.Errorf("Failed to query user balance: %v", err)
	}

	expectedBalance := initialBalance - (itemPrice * float64(buyQty))
	if newBalance != expectedBalance {
		t.Errorf("Expected balance %.2f, got %.2f", expectedBalance, newBalance)
	}

	var newStock int
	err = pool.QueryRow(ctx, "SELECT stock FROM items WHERE id = $1", itemID).Scan(&newStock)
	if err != nil {
		t.Errorf("Failed to query item stock: %v", err)
	}

	expectedStock := initialStock - buyQty
	if newStock != expectedStock {
		t.Errorf("Expected stock %d, got %d", expectedStock, newStock)
	}

	// Verify Order Created
	var orderCount int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM orders WHERE user_id = $1 AND item_id = $2", userID, itemID).Scan(&orderCount)
	if err != nil {
		t.Errorf("Failed to query orders: %v", err)
	}
	if orderCount != 1 {
		t.Errorf("Expected 1 order, got %d", orderCount)
	}
}

func TestBuyItem_InsufficientFunds(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	ctx := context.Background()

	// Seed with low balance
	pool.Exec(ctx, "INSERT INTO users (id, first_name, last_name, balance) VALUES (1, 'Poor', 'User', 5.0)")
	pool.Exec(ctx, "INSERT INTO items (id, name, price, stock) VALUES (1, 'Test Item', 10.0, 5)")

	repo := repository.NewShopRepository(pool)
	svc := service.NewShopService(repo)
	h := handler.NewShopHandler(svc)

	reqBody, _ := json.Marshal(map[string]interface{}{"user_id": 1, "item_id": 1, "count": 1})
	req := httptest.NewRequest(http.MethodPost, "/buy", bytes.NewBuffer(reqBody))
	w := httptest.NewRecorder()

	h.BuyItem(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 Bad Request, got %d", w.Code)
	}
}

func TestBuyItem_InsufficientStock(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	ctx := context.Background()

	pool.Exec(ctx, "INSERT INTO users (id, first_name, last_name, balance) VALUES (1, 'Rich', 'User', 1000.0)")
	pool.Exec(ctx, "INSERT INTO items (id, name, price, stock) VALUES (1, 'Test Item', 10.0, 1)")

	repo := repository.NewShopRepository(pool)
	svc := service.NewShopService(repo)
	h := handler.NewShopHandler(svc)

	// Buy 2 (Stock is 1)
	reqBody, _ := json.Marshal(map[string]interface{}{"user_id": 1, "item_id": 1, "count": 2})
	req := httptest.NewRequest(http.MethodPost, "/buy", bytes.NewBuffer(reqBody))
	w := httptest.NewRecorder()

	h.BuyItem(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 Bad Request, got %d", w.Code)
	}
}

func TestBuyItem_Concurrency(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	ctx := context.Background()

	// Setup: 10 items in stock, User has money for 100 items (rich user)
	// We will launch 50 goroutines. Only 10 should succeed.
	itemPrice := 10.0
	initialStock := 10
	initialBalance := 1000.0 // Enough for 100 items

	pool.Exec(ctx, "INSERT INTO users (id, first_name, last_name, balance) VALUES (1, 'Concurrent', 'User', $1)", initialBalance)
	pool.Exec(ctx, "INSERT INTO items (id, name, price, stock) VALUES (1, 'Test Item', $1, $2)", itemPrice, initialStock)

	repo := repository.NewShopRepository(pool)
	svc := service.NewShopService(repo)
	h := handler.NewShopHandler(svc)

	concurrentRequests := 50
	successCount := 0
	failCount := 0

	// Channel to collect results
	results := make(chan int, concurrentRequests)

	for i := 0; i < concurrentRequests; i++ {
		go func() {
			reqBody, _ := json.Marshal(map[string]interface{}{"user_id": 1, "item_id": 1, "count": 1})
			req := httptest.NewRequest(http.MethodPost, "/buy", bytes.NewBuffer(reqBody))
			w := httptest.NewRecorder()

			h.BuyItem(w, req)
			results <- w.Code
		}()
	}

	// Wait for all requests
	for i := 0; i < concurrentRequests; i++ {
		code := <-results
		if code == http.StatusOK {
			successCount++
		} else {
			failCount++
		}
	}

	// Verify counts
	if successCount != initialStock {
		t.Errorf("Expected %d successful purchases, got %d", initialStock, successCount)
	}
	expectedFails := concurrentRequests - initialStock
	if failCount != expectedFails {
		t.Errorf("Expected %d failed purchases, got %d", expectedFails, failCount)
	}

	// Verify DB Consistency
	var newStock int
	pool.QueryRow(ctx, "SELECT stock FROM items WHERE id = 1").Scan(&newStock)
	if newStock != 0 {
		t.Errorf("Expected stock 0, got %d", newStock)
	}

	var newBalance float64
	pool.QueryRow(ctx, "SELECT balance FROM users WHERE id = 1").Scan(&newBalance)
	expectedBalance := initialBalance - (float64(initialStock) * itemPrice)
	if newBalance != expectedBalance {
		t.Errorf("Expected balance %.2f, got %.2f", expectedBalance, newBalance)
	}
}
