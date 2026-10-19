package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// Database methods for admin controls
func (database *SQLiteDatabase) ListAllUsers(ctx context.Context) ([]Account, error) {
	rows, err := database.db.QueryContext(ctx, `
		SELECT u.email, COALESCE(a.balance_cents, ?), COALESCE(a.is_admin, u.is_admin)
		FROM users u
		LEFT JOIN accounts a ON u.email = a.email
		ORDER BY u.created_at DESC
	`, StartingBalanceCents)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []Account
	for rows.Next() {
		var a Account
		var isAdminInt int
		if err := rows.Scan(&a.Email, &a.BalanceCents, &isAdminInt); err != nil {
			return nil, err
		}
		a.IsAdmin = isAdminInt == 1
		accounts = append(accounts, a)
	}
	return accounts, rows.Err()
}

func (database *SQLiteDatabase) SetUserAdmin(ctx context.Context, email string, isAdmin bool) error {
	email = strings.ToLower(strings.TrimSpace(email))
	flag := 0
	if isAdmin {
		flag = 1
	}

	_, err := database.db.ExecContext(ctx, `UPDATE users SET is_admin = ? WHERE email = ?`, flag, email)
	if err != nil {
		return err
	}
	_, err = database.db.ExecContext(
		ctx,
		`INSERT INTO accounts (email, balance_cents, is_admin) VALUES (?, ?, ?)
		 ON CONFLICT(email) DO UPDATE SET is_admin = ?`,
		email, StartingBalanceCents, flag, flag,
	)
	return err
}

func (database *SQLiteDatabase) SetUserBalance(ctx context.Context, email string, balanceCents int64) error {
	email = strings.ToLower(strings.TrimSpace(email))
	_, err := database.db.ExecContext(
		ctx,
		`INSERT INTO accounts (email, balance_cents, is_admin) VALUES (?, ?, 0)
		 ON CONFLICT(email) DO UPDATE SET balance_cents = ?`,
		email, balanceCents, balanceCents,
	)
	return err
}

func (database *SQLiteDatabase) ListAllOrders(ctx context.Context, limit int) ([]Order, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := database.db.QueryContext(
		ctx,
		`SELECT id, email, symbol, side, quantity, price_cents, total_cents, created_at
		 FROM orders ORDER BY id DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.ID, &o.Email, &o.Symbol, &o.Side, &o.Quantity, &o.PriceCents, &o.TotalCents, &o.CreatedAt); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, rows.Err()
}

// HTTP Admin Handlers

// GET /admin/users
func adminListUsersHandler(database Database) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		users, err := database.ListAllUsers(r.Context())
		if err != nil {
			http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		type userView struct {
			Email            string  `json:"email"`
			BalanceCents     int64   `json:"balance_cents"`
			BalanceFormatted float64 `json:"balance_formatted"`
			IsAdmin          bool    `json:"is_admin"`
		}

		res := make([]userView, 0, len(users))
		for _, u := range users {
			res = append(res, userView{
				Email:            u.Email,
				BalanceCents:     u.BalanceCents,
				BalanceFormatted: float64(u.BalanceCents) / 100.0,
				IsAdmin:          u.IsAdmin,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(res)
	}
}

// POST /admin/user/balance
type adminSetBalanceRequest struct {
	TargetEmail  string `json:"target_email"`
	BalanceCents int64  `json:"balance_cents"`
}

func adminSetUserBalanceHandler(database Database) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req adminSetBalanceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if req.TargetEmail == "" || req.BalanceCents < 0 {
			http.Error(w, "Valid target_email and non-negative balance_cents required", http.StatusBadRequest)
			return
		}

		if err := database.SetUserBalance(r.Context(), req.TargetEmail, req.BalanceCents); err != nil {
			http.Error(w, "Could not update balance: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"message":       "Balance updated",
			"target_email":  req.TargetEmail,
			"balance_cents": req.BalanceCents,
		})
	}
}

// POST /admin/user/role
type adminSetRoleRequest struct {
	TargetEmail string `json:"target_email"`
	IsAdmin     bool   `json:"is_admin"`
}

func adminSetUserRoleHandler(database Database) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req adminSetRoleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if req.TargetEmail == "" {
			http.Error(w, "target_email is required", http.StatusBadRequest)
			return
		}

		if err := database.SetUserAdmin(r.Context(), req.TargetEmail, req.IsAdmin); err != nil {
			http.Error(w, "Could not update admin role: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"message":      "User role updated",
			"target_email": req.TargetEmail,
			"is_admin":     req.IsAdmin,
		})
	}
}

// POST /admin/price
type adminSetPriceRequest struct {
	Symbol     string `json:"symbol"`
	PriceCents int64  `json:"price_cents"`
}

func adminSetPriceHandler(m *Market) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req adminSetPriceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		req.Symbol = strings.ToUpper(strings.TrimSpace(req.Symbol))
		if req.Symbol == "" || req.PriceCents <= 0 {
			http.Error(w, "Valid symbol and positive price_cents required", http.StatusBadRequest)
			return
		}

		ok := m.SetPrice(req.Symbol, req.PriceCents)
		if !ok {
			http.Error(w, "Unknown symbol: "+req.Symbol, http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"message":     "Price updated successfully",
			"symbol":      req.Symbol,
			"price_cents": req.PriceCents,
		})
	}
}

// GET /admin/events
func adminListEventsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(CatalogEvents)
	}
}

// POST /admin/event/trigger
type adminTriggerSpecificEventRequest struct {
	EventID int `json:"event_id"`
}

func adminTriggerSpecificEventHandler(ee *EventEngine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req adminTriggerSpecificEventRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		ev, err := ee.TriggerSpecificEvent(req.EventID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"message": "Specific event triggered successfully",
			"event":   ev,
		})
	}
}

// GET /admin/orders
func adminListAllOrdersHandler(database Database) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		limit := 100
		if lStr := r.URL.Query().Get("limit"); lStr != "" {
			if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
				limit = l
			}
		}

		orders, err := database.ListAllOrders(r.Context(), limit)
		if err != nil {
			http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if orders == nil {
			orders = []Order{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(orders)
	}
}
