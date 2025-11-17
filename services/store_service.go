package services

import (
	"context"
	"fmt"
	"outDrinkMeAPI/internal/store"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	// stripe "github.com/stripe/stripe-go/v74"
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

// ! fx this, ïs pro_only no longer exists and many others
func (s *StoreService) PurchaseStoreItem(ctx context.Context, clerkID string, itemId string) (*store.Purchase, error) {
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

	var item store.Item
	itemQuery := `
		SELECT id, base_price, is_active
		FROM store_items
		WHERE id = $1
	`
	err = tx.QueryRow(ctx, itemQuery, itemUUID).Scan(
		&item.ID,
		&item.BasePrice,
		&item.IsActive,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("store item not found")
		}
		return nil, fmt.Errorf("failed to get store item: %w", err)
	}

	if !item.IsActive {
		return nil, fmt.Errorf("store item is not available for purchase")
	}

	var userID uuid.UUID
	userQuery := `SELECT id FROM users WHERE clerk_id = $1`
	err = s.db.QueryRow(ctx, userQuery, clerkID).Scan(&userID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Check if user already has this item in inventory
	var existingInventoryItem store.InventoryItem
	checkInventoryQuery := `
		SELECT id, quantity
		FROM user_inventory
		WHERE user_id = $1 AND item_id = $2
	`
	err = tx.QueryRow(ctx, checkInventoryQuery, userID, itemUUID).Scan(
		&existingInventoryItem.ID,
		&existingInventoryItem.Quantity,
	)

	if err != nil && err != pgx.ErrNoRows {
		return nil, fmt.Errorf("failed to check existing inventory item: %w", err)
	}

	if err == pgx.ErrNoRows {
		// User does not own this item, proceed with new purchase and add to inventory
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
			INSERT INTO user_purchases (
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

	} else {
		// User already has this item, increment quantity
		newQuantity := existingInventoryItem.Quantity + 1
		updateInventoryQuery := `
			UPDATE user_inventory
			SET quantity = $1
			WHERE id = $2
		`
		_, err = tx.Exec(ctx, updateInventoryQuery, newQuantity, existingInventoryItem.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to update inventory item quantity: %w", err)
		}

		// Create a purchase record even if just incrementing quantity, to log the purchase event
		purchase := store.Purchase{
			ID:           uuid.New(),
			UserID:       userID,
			ItemID:       &itemUUID,
			PurchaseType: "store_item_quantity_increment", // A different type to distinguish
			AmountPaid:   &item.BasePrice,                 // Still record the price paid
			Status:       "completed",
			PurchasedAt:  time.Now(),
		}

		insertPurchaseQuery := `
			INSERT INTO user_purchases (
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
			return nil, fmt.Errorf("failed to create purchase for quantity increment: %w", err)
		}

		// Commit transaction
		if err = tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("failed to commit transaction: %w", err)
		}

		return &purchase, nil
	}
}

//TODO:
// func (s *StoreService) BuyGems(ctx context.Context) (map[string][]*store.Item, error) {}
// func (s *StoreService) PurchaseGems(ctx context.Context ,clerkID string, gemsCount int) (bool, error) {
// 	var userID uuid.UUID
// 	userQuery := `SELECT id FROM users WHERE clerk_id = $1`
// 	err := s.db.QueryRow(ctx, userQuery, clerkID).Scan(&userID)
// 	if err != nil {
// 		if err == pgx.ErrNoRows {
// 			return false, fmt.Errorf("user not found")
// 		}
// 		return false, fmt.Errorf("failed to get user: %w", err)
// 	}

// params := &stripe.CheckoutSessionParams{
// 		Mode: stripe.String(string(stripe.CheckoutSessionModePayment)),
// 		LineItems: []*stripe.CheckoutSessionLineItemParams{
// 			{
// 				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
// 					Currency: stripe.String("usd"),
// 					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
// 						Name: stripe.String("Service"),
// 					},
// 					UnitAmount: stripe.Int64(3000),
// 				},
// 				Quantity: stripe.Int64(1),
// 			},
// 		},
// 		SuccessURL: stripe.String("http://localhost:3000/success"),
// 		CancelURL:  stripe.String("http://localhost:3000/cancel"),
// 	}

// 	s, err := session.New(params)
// 	if err != nil {
// 		log.Printf("session.New: %v", err)
// 	}
// 	http.Redirect(w, r, s.URL, http.StatusSeeOther)
// }
