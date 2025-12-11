package service

import (
	"context"
	"errors"
	"fsanano/go-test/internal/repository"
)

type ShopService struct {
	repo *repository.ShopRepository
}

func NewShopService(repo *repository.ShopRepository) *ShopService {
	return &ShopService{repo: repo}
}

func (s *ShopService) BuyItem(ctx context.Context, userID, itemID, quantity int) error {
	// Validate quantity
	if quantity <= 0 {
		return errors.New("quantity must be greater than 0")
	}

	return s.repo.RunAtomic(ctx, func(ctx context.Context) error {
		// 1. Get Item Price and Stock with Lock
		price, stock, err := s.repo.GetItemForUpdate(ctx, itemID)
		if err != nil {
			return err
		}

		// 2. Check Stock
		if stock < quantity {
			return errors.New("insufficient stock")
		}

		// 3. Lock User Row and Get Balance
		balance, err := s.repo.GetUserForUpdate(ctx, userID)
		if err != nil {
			return err
		}

		// 4. Check Balance
		totalPrice := price * float64(quantity)
		if balance < totalPrice {
			return errors.New("insufficient funds")
		}

		// 5. Update Balance
		if err := s.repo.UpdateUserBalance(ctx, userID, totalPrice); err != nil {
			return err
		}

		// 6. Update Stock
		if err := s.repo.UpdateItemStock(ctx, itemID, quantity); err != nil {
			return err
		}

		// 7. Create Order
		if err := s.repo.CreateOrder(ctx, userID, itemID, totalPrice, quantity); err != nil {
			return err
		}

		return nil
	})
}
