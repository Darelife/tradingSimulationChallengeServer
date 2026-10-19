package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestAdminControls(t *testing.T) {
	dbPath := "admin_test.db"
	_ = os.Remove(dbPath)
	defer os.Remove(dbPath)

	db, err := NewSQLiteDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to init db: %v", err)
	}
	defer db.Close()

	testAdminEmail := "test_admin@example.com"
	testAdminPassword := "testAdminPass123"
	adminHash, _ := bcrypt.GenerateFromPassword([]byte(testAdminPassword), bcrypt.DefaultCost)
	_ = db.CreateUser(context.Background(), testAdminEmail, string(adminHash), "127.0.0.1")
	_, _ = db.db.Exec(`INSERT INTO accounts (email, balance_cents, is_admin) VALUES (?, ?, 1)`, testAdminEmail, StartingBalanceCents)

	// Verify credentials and admin status
	hash, isAdmin, err := db.GetUserCredentials(context.Background(), testAdminEmail)
	if err != nil || !isAdmin {
		t.Fatalf("Admin account lookup failed: isAdmin=%v err=%v", isAdmin, err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(testAdminPassword)); err != nil {
		t.Fatalf("Password mismatch on test admin account")
	}
	fmt.Printf("✓ Test admin account verified\n")

	// Create a regular user for testing controls
	regularEmail := "regular@goa.bits-pilani.ac.in"
	regHash, _ := bcrypt.GenerateFromPassword([]byte("user123"), bcrypt.DefaultCost)
	_ = db.CreateUser(context.Background(), regularEmail, string(regHash), "127.0.0.1")
	_, _ = db.db.Exec(`INSERT INTO accounts (email, balance_cents, is_admin) VALUES (?, ?, 0)`, regularEmail, 50000)

	market, _ := NewMarket(db)
	eventEngine := NewEventEngine(market)

	mux := http.NewServeMux()
	mux.HandleFunc("/admin/users", requireAdmin(db, adminListUsersHandler(db)))
	mux.HandleFunc("/admin/user/balance", requireAdmin(db, adminSetUserBalanceHandler(db)))
	mux.HandleFunc("/admin/user/role", requireAdmin(db, adminSetUserRoleHandler(db)))
	mux.HandleFunc("/admin/price", requireAdmin(db, adminSetPriceHandler(market)))
	mux.HandleFunc("/admin/events", requireAdmin(db, adminListEventsHandler()))
	mux.HandleFunc("/admin/event", requireAdmin(db, adminTriggerEventHandler(eventEngine)))
	mux.HandleFunc("/admin/event/trigger", requireAdmin(db, adminTriggerSpecificEventHandler(eventEngine)))
	mux.HandleFunc("/admin/orders", requireAdmin(db, adminListAllOrdersHandler(db)))

	server := httptest.NewServer(mux)
	defer server.Close()

	client := server.Client()

	// Helper for admin POST requests
	postAdmin := func(endpoint string, payload any) (*http.Response, []byte) {
		m := map[string]any{
			"email":    testAdminEmail,
			"password": testAdminPassword,
		}
		if payload != nil {
			b, _ := json.Marshal(payload)
			_ = json.Unmarshal(b, &m)
		}
		jsonBytes, _ := json.Marshal(m)
		resp, err := client.Post(server.URL+endpoint, "application/json", bytes.NewReader(jsonBytes))
		if err != nil {
			t.Fatalf("Request to %s failed: %v", endpoint, err)
		}
		body, _ := io.ReadAll(resp.Body)
		return resp, body
	}

	// Helper for admin GET requests
	getAdmin := func(endpoint string) (*http.Response, []byte) {
		req, _ := http.NewRequest(http.MethodGet, server.URL+endpoint, nil)
		req.SetBasicAuth(testAdminEmail, testAdminPassword)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("GET %s failed: %v", endpoint, err)
		}
		body, _ := io.ReadAll(resp.Body)
		return resp, body
	}

	// 1. Test GET /admin/users
	resp, body := getAdmin("/admin/users")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/admin/users returned %d: %s", resp.StatusCode, string(body))
	}
	var users []map[string]any
	_ = json.Unmarshal(body, &users)
	if len(users) < 2 {
		t.Fatalf("Expected at least 2 users, got %d", len(users))
	}
	fmt.Printf("✓ /admin/users listed %d users\n", len(users))

	// 2. Test POST /admin/user/balance (update balance to 5,000,000 paise)
	resp, body = postAdmin("/admin/user/balance", map[string]any{
		"target_email":  regularEmail,
		"balance_cents": 5000000,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/admin/user/balance returned %d: %s", resp.StatusCode, string(body))
	}
	acct, _ := db.GetAccount(context.Background(), regularEmail)
	if acct.BalanceCents != 5000000 {
		t.Fatalf("Expected balance 5000000, got %d", acct.BalanceCents)
	}
	fmt.Printf("✓ /admin/user/balance updated balance to %d paise\n", acct.BalanceCents)

	// 3. Test POST /admin/user/role (promote user to admin)
	resp, body = postAdmin("/admin/user/role", map[string]any{
		"target_email": regularEmail,
		"is_admin":     true,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/admin/user/role returned %d: %s", resp.StatusCode, string(body))
	}
	acct, _ = db.GetAccount(context.Background(), regularEmail)
	if !acct.IsAdmin {
		t.Fatalf("Expected user to be admin")
	}
	fmt.Printf("✓ /admin/user/role successfully promoted user to admin\n")

	// 4. Test POST /admin/price (override price of NEXA to 300,000 paise)
	resp, body = postAdmin("/admin/price", map[string]any{
		"symbol":      "NEXA",
		"price_cents": 300000,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/admin/price returned %d: %s", resp.StatusCode, string(body))
	}
	newPrice, _ := market.GetPrice("NEXA")
	if newPrice != 300000 {
		t.Fatalf("Expected price 300000, got %d", newPrice)
	}
	fmt.Printf("✓ /admin/price updated NEXA price to %d paise\n", newPrice)

	// 5. Test GET /admin/events (catalog of 100 events)
	resp, body = getAdmin("/admin/events")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/admin/events returned %d: %s", resp.StatusCode, string(body))
	}
	var catalog []MarketEvent
	_ = json.Unmarshal(body, &catalog)
	if len(catalog) != 100 {
		t.Fatalf("Expected 100 events in catalog, got %d", len(catalog))
	}
	fmt.Printf("✓ /admin/events verified: 100 events in catalog\n")

	// 6. Test POST /admin/event/trigger (trigger specific event #10)
	resp, body = postAdmin("/admin/event/trigger", map[string]any{
		"event_id": 10,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/admin/event/trigger returned %d: %s", resp.StatusCode, string(body))
	}
	activeEv := eventEngine.GetActive()
	if activeEv == nil || activeEv.Event.ID != 10 {
		t.Fatalf("Expected active event #10, got %+v", activeEv)
	}
	fmt.Printf("✓ /admin/event/trigger fired specific event #10: %s\n", activeEv.Event.Description)

	// 7. Test GET /admin/orders
	resp, body = getAdmin("/admin/orders")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/admin/orders returned %d: %s", resp.StatusCode, string(body))
	}
	fmt.Printf("✓ /admin/orders returned HTTP 200\n")
}
