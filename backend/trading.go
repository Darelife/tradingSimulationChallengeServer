package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (database *SQLiteDatabase) GetAccount(ctx context.Context, email string) (Account, error) {
	var acct Account
	var isAdminInt int

	err := database.db.QueryRowContext(
		ctx,
		`SELECT email, balance_cents, is_admin FROM accounts WHERE email = ?`,
		strings.ToLower(email),
	).Scan(&acct.Email, &acct.BalanceCents, &isAdminInt)

	if errors.Is(err, sql.ErrNoRows) {
		// If user exists in users table, lazily initialize account
		exists, checkErr := database.UserExists(ctx, email)
		if checkErr == nil && exists {
			adminFlag := 0
			adminEmail := strings.ToLower(strings.TrimSpace(databaseGetAdminEmail()))
			if adminEmail != "" && strings.ToLower(email) == adminEmail {
				adminFlag = 1
			}
			_, insertErr := database.db.ExecContext(
				ctx,
				`INSERT OR IGNORE INTO accounts (email, balance_cents, is_admin) VALUES (?, ?, ?)`,
				strings.ToLower(email),
				StartingBalanceCents,
				adminFlag,
			)
			if insertErr == nil {
				return Account{
					Email:        email,
					BalanceCents: StartingBalanceCents,
					IsAdmin:      adminFlag == 1,
				}, nil
			}
		}
		return acct, sql.ErrNoRows
	}
	if err != nil {
		return acct, err
	}

	acct.IsAdmin = isAdminInt == 1
	return acct, nil
}

func (database *SQLiteDatabase) ExecuteTrade(
	ctx context.Context,
	email string,
	symbol string,
	side string,
	quantity int64,
	priceCents int64,
) (Order, error) {
	var order Order
	if quantity <= 0 {
		return order, ErrInvalidQuantity
	}
	if priceCents <= 0 {
		return order, errors.New("invalid execution price")
	}

	side = strings.ToLower(side)
	if side != "buy" && side != "sell" {
		return order, errors.New("side must be 'buy' or 'sell'")
	}

	totalCents := quantity * priceCents

	tx, err := database.db.BeginTx(ctx, nil)
	if err != nil {
		return order, err
	}
	defer tx.Rollback()

	// Ensure account exists
	var currentBalance int64
	var isAdmin int
	err = tx.QueryRowContext(
		ctx,
		`SELECT balance_cents, is_admin FROM accounts WHERE email = ?`,
		email,
	).Scan(&currentBalance, &isAdmin)

	if errors.Is(err, sql.ErrNoRows) {
		currentBalance = StartingBalanceCents
		_, err = tx.ExecContext(
			ctx,
			`INSERT INTO accounts (email, balance_cents, is_admin) VALUES (?, ?, 0)`,
			email,
			currentBalance,
		)
		if err != nil {
			return order, err
		}
	} else if err != nil {
		return order, err
	}

	// Fetch holding
	var currentQty int64
	var avgCost int64
	err = tx.QueryRowContext(
		ctx,
		`SELECT quantity, avg_cost_cents FROM holdings WHERE email = ? AND symbol = ?`,
		email,
		symbol,
	).Scan(&currentQty, &avgCost)

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return order, err
	}

	if side == "buy" {
		if currentBalance < totalCents {
			return order, ErrInsufficientFunds
		}
		newBalance := currentBalance - totalCents
		_, err = tx.ExecContext(ctx, `UPDATE accounts SET balance_cents = ? WHERE email = ?`, newBalance, email)
		if err != nil {
			return order, err
		}

		// Recompute average cost
		newQty := currentQty + quantity
		newAvgCost := (currentQty*avgCost + totalCents) / newQty

		_, err = tx.ExecContext(
			ctx,
			`INSERT INTO holdings (email, symbol, quantity, avg_cost_cents)
			 VALUES (?, ?, ?, ?)
			 ON CONFLICT(email, symbol) DO UPDATE SET quantity = ?, avg_cost_cents = ?`,
			email, symbol, newQty, newAvgCost,
			newQty, newAvgCost,
		)
		if err != nil {
			return order, err
		}
	} else { // sell
		if currentQty < quantity {
			return order, ErrInsufficientShares
		}
		newBalance := currentBalance + totalCents
		_, err = tx.ExecContext(ctx, `UPDATE accounts SET balance_cents = ? WHERE email = ?`, newBalance, email)
		if err != nil {
			return order, err
		}

		newQty := currentQty - quantity
		if newQty == 0 {
			_, err = tx.ExecContext(ctx, `DELETE FROM holdings WHERE email = ? AND symbol = ?`, email, symbol)
		} else {
			_, err = tx.ExecContext(ctx, `UPDATE holdings SET quantity = ? WHERE email = ? AND symbol = ?`, newQty, email, symbol)
		}
		if err != nil {
			return order, err
		}
	}

	now := time.Now().Unix()
	res, err := tx.ExecContext(
		ctx,
		`INSERT INTO orders (email, symbol, side, quantity, price_cents, total_cents, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		email, symbol, side, quantity, priceCents, totalCents, now,
	)
	if err != nil {
		return order, err
	}

	orderID, _ := res.LastInsertId()
	if err := tx.Commit(); err != nil {
		return order, err
	}

	order = Order{
		ID:         orderID,
		Email:      email,
		Symbol:     symbol,
		Side:       side,
		Quantity:   quantity,
		PriceCents: priceCents,
		TotalCents: totalCents,
		CreatedAt:  now,
	}
	return order, nil
}

func (database *SQLiteDatabase) ListOrders(ctx context.Context, email string, limit int) ([]Order, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := database.db.QueryContext(
		ctx,
		`SELECT id, symbol, side, quantity, price_cents, total_cents, created_at
		 FROM orders WHERE email = ? ORDER BY id DESC LIMIT ?`,
		strings.ToLower(email),
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var o Order
		o.Email = email
		if err := rows.Scan(&o.ID, &o.Symbol, &o.Side, &o.Quantity, &o.PriceCents, &o.TotalCents, &o.CreatedAt); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, rows.Err()
}

func (database *SQLiteDatabase) ListHoldings(ctx context.Context, email string) ([]Holding, error) {
	rows, err := database.db.QueryContext(
		ctx,
		`SELECT symbol, quantity, avg_cost_cents FROM holdings WHERE email = ? AND quantity > 0 ORDER BY symbol ASC`,
		strings.ToLower(email),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var holdings []Holding
	for rows.Next() {
		var h Holding
		if err := rows.Scan(&h.Symbol, &h.Quantity, &h.AvgCostCents); err != nil {
			return nil, err
		}
		holdings = append(holdings, h)
	}
	return holdings, rows.Err()
}

func databaseGetAdminEmail() string {
	// reads from env or empty
	return ""
}

type tradeRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Symbol   string `json:"symbol"`
	Quantity int64  `json:"quantity"`
}

func buyHandler(database Database, m *Market) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		email, ok := r.Context().Value(userEmailContextKey).(string)
		if !ok || email == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var req tradeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		sym := strings.ToUpper(strings.TrimSpace(req.Symbol))
		price, ok := m.GetPrice(sym)
		if !ok {
			http.Error(w, "Unknown symbol", http.StatusBadRequest)
			return
		}

		order, err := database.ExecuteTrade(r.Context(), email, sym, "buy", req.Quantity, price)
		if err != nil {
			if errors.Is(err, ErrInsufficientFunds) || errors.Is(err, ErrInvalidQuantity) {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			http.Error(w, fmt.Sprintf("Trade error: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(order)
	}
}

func sellHandler(database Database, m *Market) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		email, ok := r.Context().Value(userEmailContextKey).(string)
		if !ok || email == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var req tradeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		sym := strings.ToUpper(strings.TrimSpace(req.Symbol))
		price, ok := m.GetPrice(sym)
		if !ok {
			http.Error(w, "Unknown symbol", http.StatusBadRequest)
			return
		}

		order, err := database.ExecuteTrade(r.Context(), email, sym, "sell", req.Quantity, price)
		if err != nil {
			if errors.Is(err, ErrInsufficientShares) || errors.Is(err, ErrInvalidQuantity) {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			http.Error(w, fmt.Sprintf("Trade error: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(order)
	}
}

func ordersHandler(database Database) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		email, ok := r.Context().Value(userEmailContextKey).(string)
		if !ok || email == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		limit := 50
		if lStr := r.URL.Query().Get("limit"); lStr != "" {
			if parsed, err := strconv.Atoi(lStr); err == nil && parsed > 0 {
				limit = parsed
			}
		}

		orders, err := database.ListOrders(r.Context(), email, limit)
		if err != nil {
			http.Error(w, "Could not fetch orders", http.StatusInternalServerError)
			return
		}
		if orders == nil {
			orders = []Order{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(orders)
	}
}

func portfolioHandler(database Database, m *Market) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		email, ok := r.Context().Value(userEmailContextKey).(string)
		if !ok || email == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		acct, err := database.GetAccount(r.Context(), email)
		if err != nil {
			http.Error(w, "Could not fetch account", http.StatusInternalServerError)
			return
		}

		holdings, err := database.ListHoldings(r.Context(), email)
		if err != nil {
			http.Error(w, "Could not fetch holdings", http.StatusInternalServerError)
			return
		}

		type enrichedHolding struct {
			Symbol          string  `json:"symbol"`
			Quantity        int64   `json:"quantity"`
			AvgCostCents    int64   `json:"avg_cost_cents"`
			CurrentPrice    int64   `json:"current_price_cents"`
			TotalValueCents int64   `json:"total_value_cents"`
			PnlCents        int64   `json:"pnl_cents"`
		}

		var enriched []enrichedHolding
		var portfolioSecuritiesValue int64

		for _, h := range holdings {
			price, _ := m.GetPrice(h.Symbol)
			val := h.Quantity * price
			pnl := val - (h.Quantity * h.AvgCostCents)
			portfolioSecuritiesValue += val
			enriched = append(enriched, enrichedHolding{
				Symbol:          h.Symbol,
				Quantity:        h.Quantity,
				AvgCostCents:    h.AvgCostCents,
				CurrentPrice:    price,
				TotalValueCents: val,
				PnlCents:        pnl,
			})
		}

		if enriched == nil {
			enriched = []enrichedHolding{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"balance_cents":            acct.BalanceCents,
			"balance_formatted":        float64(acct.BalanceCents) / 100.0,
			"holdings":                 enriched,
			"total_portfolio_value":    acct.BalanceCents + portfolioSecuritiesValue,
		})
	}
}
