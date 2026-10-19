package main

import (
	"fmt"
	"log"
	"net/http"
)

func home(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Trading Simulation Challenge Server is running")
}

func main() {
	database, err := NewSQLiteDatabase("trading.db")
	if err != nil {
		panic(err)
	}
	defer database.Close()


	// Initialize market simulation engine with 6 companies and historical data
	market, err := NewMarket(database)
	if err != nil {
		log.Fatalf("Failed to initialize market engine: %v", err)
	}

	// Initialize events engine and virtual traders
	eventEngine := NewEventEngine(market)

	// Rate limits:
	// - Auth start: 2 requests/sec, burst 5
	// - Auth complete: 5 requests/sec, burst 10
	// - Trading endpoints: 30 requests/sec, burst 60
	// - Market prices/history: 60 requests/sec, burst 120
	// - Admin trigger: 2 requests/sec, burst 5

	http.HandleFunc("/", home)

	// User registration & password reset flows
	http.HandleFunc("/register/start", rateLimit("register_start", 2.0, 5.0, registerStart(database)))
	http.HandleFunc("/register/complete", rateLimit("register_complete", 5.0, 10.0, registerComplete(database)))
	http.HandleFunc("/forgot-password/start", rateLimit("forgot_start", 2.0, 5.0, forgotPasswordStart(database)))
	http.HandleFunc("/forgot-password/complete", rateLimit("forgot_complete", 5.0, 10.0, forgotPasswordComplete(database)))


	// Market information endpoints
	http.HandleFunc("/price", rateLimit("price", 60.0, 120.0, priceHandler(market)))
	http.HandleFunc("/price/history", rateLimit("history", 60.0, 120.0, historyHandler(database)))

	// Trading & order book endpoints (Protected by session auth)
	http.HandleFunc("/buy", rateLimit("buy", 30.0, 60.0, requireAuth(database, buyHandler(database, market))))
	http.HandleFunc("/sell", rateLimit("sell", 30.0, 60.0, requireAuth(database, sellHandler(database, market))))
	http.HandleFunc("/orders", rateLimit("orders", 30.0, 60.0, requireAuth(database, ordersHandler(database))))
	http.HandleFunc("/portfolio", rateLimit("portfolio", 30.0, 60.0, requireAuth(database, portfolioHandler(database, market))))

	// Events endpoints
	http.HandleFunc("/event", rateLimit("event", 60.0, 120.0, eventHandler(eventEngine)))
	// Admin Controls (Protected by admin email/password check)
	http.HandleFunc("/admin/event", rateLimit("admin_event", 5.0, 10.0, requireAdmin(database, adminTriggerEventHandler(eventEngine))))
	http.HandleFunc("/admin/events", rateLimit("admin_events", 10.0, 20.0, requireAdmin(database, adminListEventsHandler())))
	http.HandleFunc("/admin/event/trigger", rateLimit("admin_event_trigger", 5.0, 10.0, requireAdmin(database, adminTriggerSpecificEventHandler(eventEngine))))
	http.HandleFunc("/admin/users", rateLimit("admin_users", 10.0, 20.0, requireAdmin(database, adminListUsersHandler(database))))
	http.HandleFunc("/admin/user/balance", rateLimit("admin_user_balance", 5.0, 10.0, requireAdmin(database, adminSetUserBalanceHandler(database))))
	http.HandleFunc("/admin/user/role", rateLimit("admin_user_role", 5.0, 10.0, requireAdmin(database, adminSetUserRoleHandler(database))))
	http.HandleFunc("/admin/price", rateLimit("admin_price", 10.0, 20.0, requireAdmin(database, adminSetPriceHandler(market))))
	http.HandleFunc("/admin/orders", rateLimit("admin_orders", 10.0, 20.0, requireAdmin(database, adminListAllOrdersHandler(database))))
	fmt.Println("Listening on http://localhost:8080")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
