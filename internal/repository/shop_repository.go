package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ShopRepository struct {
	db *pgxpool.Pool
}

func NewShopRepository(db *pgxpool.Pool) *ShopRepository {
	return &ShopRepository{db: db}
}

// RunAtomic executes a function within a transaction
func (r *ShopRepository) RunAtomic(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	// Defer rollback in case of panic or error (if commit succeeds, rollback does nothing)
	defer tx.Rollback(ctx)

	// Inject tx into context
	// NOTE: meaningful context key or pass tx explicitly?
	// For simplicity in this project (as `db *pgxpool.Pool` is in struct),
	// we assume we just pass context. BUT `pgx` handles tx via `BeginTx` or using logic on connection.
	// However, `pgxpool` methods usually take context. To run queries INSIDE tx, we need the `pgx.Tx` object.
	//
	// A common pattern is interface for Querier (Exec, QueryRow, etc).
	// To keep it simple and Clean Architecture compliant without robust `context.WithValue` magic complications:
	// We will pass `ctx` hoping the methods use `db`? NO.
	//
	// `RunAtomic` usually needs to provide a way for called methods to use the TX.
	//
	// APPROACH:
	// The methods `GetItemForUpdate` etc. currently use `r.db.QueryRow`.
	// We need them to be able to use EITHER `r.db` OR a `tx`.
	//
	// Let's use a context key for the transaction, or a closure based approach where we pass an interface?
	//
	// Simplest approach for now: Store Tx in context?
	// `pgx` doesn't automatically pick it up.
	//
	// Let's implement `db` extraction helper or just pass `tx`?
	// But `ShopService` shouldn't know about `pgx.Tx`.
	//
	// Refined Plan (Standard Go Pattern):
	// Define a `Querier` interface? Or use Context.
	//
	// context approach:

	ctx = context.WithValue(ctx, txKey{}, tx)

	if err := fn(ctx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

type txKey struct{}

func (r *ShopRepository) getExecutor(ctx context.Context) PgxExecutor {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return r.db
}

// PgxExecutor is an interface that matches both *pgx.Conn/Pool and pgx.Tx
type PgxExecutor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (commandTag pgconn.CommandTag, err error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// GetItemForUpdate locks the item row and returns item data
func (r *ShopRepository) GetItemForUpdate(ctx context.Context, itemID int) (float64, int, error) {
	var price float64
	var stock int
	err := r.getExecutor(ctx).QueryRow(ctx, "SELECT price, stock FROM items WHERE id = $1 FOR UPDATE", itemID).Scan(&price, &stock)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, 0, errors.New("item not found")
		}
		return 0, 0, fmt.Errorf("failed to get item: %w", err)
	}
	return price, stock, nil
}

// GetUserForUpdate locks the user row and returns balance
func (r *ShopRepository) GetUserForUpdate(ctx context.Context, userID int) (float64, error) {
	var balance float64
	err := r.getExecutor(ctx).QueryRow(ctx, "SELECT balance FROM users WHERE id = $1 FOR UPDATE", userID).Scan(&balance)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, errors.New("user not found")
		}
		return 0, fmt.Errorf("failed to get user balance: %w", err)
	}
	return balance, nil
}

// UpdateItemStock updates the stock of an item
func (r *ShopRepository) UpdateItemStock(ctx context.Context, itemID int, quantity int) error {
	_, err := r.getExecutor(ctx).Exec(ctx, "UPDATE items SET stock = stock - $1 WHERE id = $2", quantity, itemID)
	if err != nil {
		return fmt.Errorf("failed to update item stock: %w", err)
	}
	return nil
}

// UpdateUserBalance updates the balance of a user
func (r *ShopRepository) UpdateUserBalance(ctx context.Context, userID int, amount float64) error {
	_, err := r.getExecutor(ctx).Exec(ctx, "UPDATE users SET balance = balance - $1 WHERE id = $2", amount, userID)
	if err != nil {
		return fmt.Errorf("failed to update user balance: %w", err)
	}
	return nil
}

// CreateOrder inserts a new order
func (r *ShopRepository) CreateOrder(ctx context.Context, userID, itemID int, price float64, quantity int) error {
	_, err := r.getExecutor(ctx).Exec(ctx, "INSERT INTO orders (user_id, item_id, price, quantity) VALUES ($1, $2, $3, $4)", userID, itemID, price, quantity)
	if err != nil {
		return fmt.Errorf("failed to create order: %w", err)
	}
	return nil
}
