package services

import (
	"context"
	"fmt"
	"outDrinkMeAPI/internal/store"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type StoreService struct {
	db *pgxpool.Pool
}

func NewStoreService(db *pgxpool.Pool) *StoreService {
	return &StoreService{db: db}
}

func (s *StoreService) GetStore(ctx context.Context) (map[string][]*store.Item, error) {
	query := `
    SELECT
        s.id,
        s.category_id,
        s.name,
        s.description,
        s.item_type,
        s.image_url,
        s.base_price,
        s.is_pro_only,
        s.is_active
    FROM store_items s
    `

	rows, err := s.db.Query(ctx, query)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var all_store_items = make(map[string][]*store.Item)
	for rows.Next() {
		var item store.Item
		err := rows.Scan(
			&item.ID,
			&item.CategoryID,
			&item.Name,
			&item.Description,
			&item.ItemType,
			&item.ImageURL,
			&item.BasePrice,
			&item.IsProOnly,
			&item.IsActive,
		)
		if err != nil {
			return nil, err
		}
		all_store_items[item.ItemType] = append(all_store_items[item.ItemType], &item)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return all_store_items, nil
}


func (s *StoreService) PurchaseStoreItem(ctx context.Context, clerkId string, itemId string) (*store.Purchase, error) {
	// Start a transaction
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Parse itemId to UUID
	itemUUID, err := uuid.Parse(itemId)
	if err != nil {
		return nil, fmt.Errorf("invalid item ID: %w", err)
	}

	// Get the store item details
	var item store.Item
	itemQuery := `
		SELECT id, base_price, is_pro_only, is_active
		FROM store_items
		WHERE id = $1
	`
	err = tx.QueryRow(ctx, itemQuery, itemUUID).Scan(
		&item.ID,
		&item.BasePrice,
		&item.IsProOnly,
		&item.IsActive,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("store item not found")
		}
		return nil, fmt.Errorf("failed to get store item: %w", err)
	}

	// Check if item is active
	if !item.IsActive {
		return nil, fmt.Errorf("store item is not available for purchase")
	}

	// Get user ID from clerk ID
	var userID uuid.UUID
	userQuery := `SELECT id FROM users WHERE clerk_id = $1`
	err = tx.QueryRow(ctx, userQuery, clerkId).Scan(&userID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Check if user already owns this item
	var existingPurchase int
	checkQuery := `
		SELECT COUNT(*)
		FROM purchases
		WHERE user_id = $1 AND item_id = $2 AND status = 'completed'
	`
	err = tx.QueryRow(ctx, checkQuery, userID, itemUUID).Scan(&existingPurchase)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing purchase: %w", err)
	}
	if existingPurchase > 0 {
		return nil, fmt.Errorf("user already owns this item")
	}

	// Create purchase record
	purchase := store.Purchase{
		ID:           uuid.New(),
		UserID:       userID,
		ItemID:       &itemUUID,
		PurchaseType: "store_item",
		AmountPaid:   &item.BasePrice,
		Status:       "completed",
		PurchasedAt:  time.Now(),
	}

	insertPurchaseQuery := `
		INSERT INTO purchases (
			id, user_id, item_id, purchase_type, amount_paid, 
			currency, transaction_id, status, purchased_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err = tx.Exec(ctx, insertPurchaseQuery,
		purchase.ID,
		purchase.UserID,
		purchase.ItemID,
		purchase.PurchaseType,
		purchase.AmountPaid,
		purchase.Currency,
		purchase.TransactionID,
		purchase.Status,
		purchase.PurchasedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create purchase: %w", err)
	}

	// Add item to user's inventory
	inventoryItem := store.InventoryItem{
		ID:         uuid.New(),
		UserID:     userID,
		ItemID:     itemUUID,
		Quantity:   1,
		IsEquipped: false,
		AcquiredAt: time.Now(),
		ExpiresAt:  nil,
	}

	insertInventoryQuery := `
		INSERT INTO user_inventory (
			id, user_id, item_id, quantity, is_equipped, acquired_at, expires_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err = tx.Exec(ctx, insertInventoryQuery,
		inventoryItem.ID,
		inventoryItem.UserID,
		inventoryItem.ItemID,
		inventoryItem.Quantity,
		inventoryItem.IsEquipped,
		inventoryItem.AcquiredAt,
		inventoryItem.ExpiresAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to add item to inventory: %w", err)
	}

	// Commit transaction
	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &purchase, nil
}


//TODO:
// func (s *StoreService) BuyGems(ctx context.Context) (map[string][]*store.Item, error) {}
