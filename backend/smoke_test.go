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
	"time"

	"golang.org/x/crypto/bcrypt"
)

func TestEndToEndSimulation(t *testing.T) {
	dbPath := "smoke_test.db"
	_ = os.Remove(dbPath)
	defer os.Remove(dbPath)

	db, err := NewSQLiteDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to init test db: %v", err)
	}
	defer db.Close()

	pwHash, _ := bcrypt.GenerateFromPassword([]byte("testpass123"), bcrypt.DefaultCost)
	adminEmail := "admin@goa.bits-pilani.ac.in"
	userEmail := "trader@goa.bits-pilani.ac.in"

	_ = db.CreateUser(context.Background(), adminEmail, string(pwHash), "127.0.0.1")
	_ = db.CreateUser(context.Background(), userEmail, string(pwHash), "127.0.0.1")

	_, _ = db.db.Exec(`INSERT INTO accounts (email, balance_cents, is_admin) VALUES (?, ?, 1)`, adminEmail, StartingBalanceCents)
	_, _ = db.db.Exec(`INSERT INTO accounts (email, balance_cents, is_admin) VALUES (?, ?, 0)`, userEmail, StartingBalanceCents)

	market, err := NewMarket(db)
	if err != nil {
		t.Fatalf("Failed to init market: %v", err)
	}

	eventEngine := NewEventEngine(market)

	mux := http.NewServeMux()
	mux.HandleFunc("/price", priceHandler(market))
	mux.HandleFunc("/price/history", historyHandler(db))
	mux.HandleFunc("/buy", requireAuth(db, buyHandler(db, market)))
	mux.HandleFunc("/sell", requireAuth(db, sellHandler(db, market)))
	mux.HandleFunc("/orders", requireAuth(db, ordersHandler(db)))
	mux.HandleFunc("/portfolio", requireAuth(db, portfolioHandler(db, market)))
	mux.HandleFunc("/event", eventHandler(eventEngine))
	mux.HandleFunc("/admin/event", requireAdmin(db, adminTriggerEventHandler(eventEngine)))

	server := httptest.NewServer(mux)
	defer server.Close()

	client := server.Client()

	// 1. Check price API
	resp, err := client.Get(server.URL + "/price")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /price failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	var prices []map[string]any
	if err := json.Unmarshal(body, &prices); err != nil || len(prices) != 6 {
		t.Fatalf("Expected 6 companies in prices, got %d", len(prices))
	}
	fmt.Printf("✓ Price API verified: 6 companies loaded\n")

	// 2. Check history API for VOLT
	resp, err = client.Get(server.URL + "/price/history?symbol=VOLT")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /price/history failed: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	var history []PricePoint
	if err := json.Unmarshal(body, &history); err != nil || len(history) < 100 {
		t.Fatalf("Expected >100 historical points for VOLT, got %d", len(history))
	}
	fmt.Printf("✓ History API verified: %d historical points loaded for VOLT\n", len(history))

	// 3. Unauthenticated buy attempt fails
	unauthBuy, _ := json.Marshal(map[string]any{
		"symbol":   "VOLT",
		"quantity": 10,
	})
	resp, err = client.Post(server.URL+"/buy", "application/json", bytes.NewReader(unauthBuy))
	if err != nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("Expected 401 Unauthorized without credentials, got %d", resp.StatusCode)
	}
	fmt.Printf("✓ Unauthenticated buy request rejected (401)\n")

	// 4. Buy 10 VOLT shares supplying email & password in JSON payload
	buyBody, _ := json.Marshal(map[string]any{
		"email":    userEmail,
		"password": "testpass123",
		"symbol":   "VOLT",
		"quantity": 10,
	})
	resp, err = client.Post(server.URL+"/buy", "application/json", bytes.NewReader(buyBody))
	if err != nil || resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("Buy failed (%d): %s", resp.StatusCode, string(b))
	}
	var buyOrder Order
	body, _ = io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &buyOrder)
	if buyOrder.Quantity != 10 || buyOrder.Symbol != "VOLT" {
		t.Fatalf("Unexpected buy order: %+v", buyOrder)
	}
	fmt.Printf("✓ Executed BUY order with email/password payload: %d shares of %s at %d paise\n", buyOrder.Quantity, buyOrder.Symbol, buyOrder.PriceCents)

	// 5. Check Portfolio supplying credentials via Basic Auth
	req, _ := http.NewRequest(http.MethodGet, server.URL+"/portfolio", nil)
	req.SetBasicAuth(userEmail, "testpass123")
	resp, err = client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("Portfolio failed: status %d", resp.StatusCode)
	}
	var pf map[string]any
	body, _ = io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &pf)
	holdingsList := pf["holdings"].([]any)
	if len(holdingsList) != 1 {
		t.Fatalf("Expected 1 holding, got %d", len(holdingsList))
	}
	fmt.Printf("✓ Portfolio verified via Basic Auth with balance: %v paise\n", pf["balance_cents"])

	// 6. Sell 5 VOLT shares with email & password in payload
	sellBody, _ := json.Marshal(map[string]any{
		"email":    userEmail,
		"password": "testpass123",
		"symbol":   "VOLT",
		"quantity": 5,
	})
	resp, err = client.Post(server.URL+"/sell", "application/json", bytes.NewReader(sellBody))
	if err != nil || resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("Sell failed (%d): %s", resp.StatusCode, string(b))
	}
	fmt.Printf("✓ Executed SELL order: 5 shares of VOLT\n")

	// 7. List Orders with credentials in query params
	resp, err = client.Get(fmt.Sprintf("%s/orders?email=%s&password=%s", server.URL, userEmail, "testpass123"))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("List orders failed: %d", resp.StatusCode)
	}
	var ordersList []Order
	body, _ = io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &ordersList)
	if len(ordersList) != 2 {
		t.Fatalf("Expected 2 orders, got %d", len(ordersList))
	}
	fmt.Printf("✓ Orders verified via query param credentials: 2 orders listed (1 BUY, 1 SELL)\n")

	// 8. Test Event Trigger (Forbidden for standard trader with correct password)
	adminTriggerBody, _ := json.Marshal(map[string]string{
		"email":    userEmail,
		"password": "testpass123",
	})
	resp, _ = client.Post(server.URL+"/admin/event", "application/json", bytes.NewReader(adminTriggerBody))
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for non-admin on /admin/event, got %d", resp.StatusCode)
	}
	fmt.Printf("✓ Admin authorization guard verified (non-admin forbidden)\n")

	// Trigger random event as admin user
	adminReqBody, _ := json.Marshal(map[string]string{
		"email":    adminEmail,
		"password": "testpass123",
	})
	voltPre, _ := market.GetPrice("VOLT")
	fuelPre, _ := market.GetPrice("FUEL")

	resp, err = client.Post(server.URL+"/admin/event", "application/json", bytes.NewReader(adminReqBody))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("Admin event trigger failed: %d", resp.StatusCode)
	}
	var evResult map[string]any
	body, _ = io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &evResult)
	evMap := evResult["event"].(map[string]any)
	fmt.Printf("✓ Triggered Event #%v: %s (alpha: %v, symbol: %s)\n", evMap["id"], evMap["description"], evMap["alpha"], evMap["symbol"])

	// Check public /event endpoint
	resp, _ = client.Get(server.URL + "/event")
	body, _ = io.ReadAll(resp.Body)
	var activeEv map[string]any
	_ = json.Unmarshal(body, &activeEv)
	if activeEv["active"] != true {
		t.Fatalf("Expected active event on GET /event")
	}
	fmt.Printf("✓ GET /event displays active event description\n")

	time.Sleep(2200 * time.Millisecond)
	voltPost, _ := market.GetPrice("VOLT")
	fuelPost, _ := market.GetPrice("FUEL")
	fmt.Printf("✓ Live price delta under virtual trader reaction: VOLT %d -> %d | FUEL %d -> %d\n", voltPre, voltPost, fuelPre, fuelPost)
}
