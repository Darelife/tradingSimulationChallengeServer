package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"math"
	"math/rand/v2"
	"net/http"
	"strconv"
	"sync"
	"time"
)
// Initial fixed companies.
// VOLT and FUEL are complementary stocks (EV transition vs fossil fuel demand).
var InitialCompanies = []Company{
	{Symbol: "NEXA", Name: "Nexatech Cloud Systems", BasePriceCents: 245000},
	{Symbol: "ORGA", Name: "Organica Green Foods", BasePriceCents: 18000},
	{Symbol: "VOLT", Name: "Voltmobility Electric", BasePriceCents: 89000, ComplementaryOf: "FUEL"},
	{Symbol: "FUEL", Name: "Fuelcore Petrochemicals", BasePriceCents: 34000, ComplementaryOf: "VOLT"},
	{Symbol: "MEDC", Name: "Medcure Pharmaceuticals", BasePriceCents: 120000},
	{Symbol: "AERO", Name: "Aerosky Logistics", BasePriceCents: 56000},
}

// Market maintains the live prices in memory and persists snapshots.
type Market struct {
	database  Database
	mu        sync.RWMutex
	prices    map[string]int64
	companies map[string]Company
	sentiment map[string]float64 // net virtual buy/sell sentiment pressure (-1.0 to +1.0)
	rng       *rand.Rand
}

func (database *SQLiteDatabase) ListCompanies(ctx context.Context) ([]Company, error) {
	rows, err := database.db.QueryContext(ctx, `SELECT symbol, name, COALESCE(complementary_of, ''), base_price_cents FROM companies ORDER BY symbol`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var companies []Company
	for rows.Next() {
		var c Company
		if err := rows.Scan(&c.Symbol, &c.Name, &c.ComplementaryOf, &c.BasePriceCents); err != nil {
			return nil, err
		}
		companies = append(companies, c)
	}
	return companies, rows.Err()
}

func (database *SQLiteDatabase) SavePricePoint(ctx context.Context, point PricePoint) error {
	_, err := database.db.ExecContext(
		ctx,
		`INSERT OR REPLACE INTO price_points (symbol, ts, price_cents) VALUES (?, ?, ?)`,
		point.Symbol,
		point.Timestamp,
		point.PriceCents,
	)
	return err
}

func (database *SQLiteDatabase) SavePricePoints(ctx context.Context, points []PricePoint) error {
	tx, err := database.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `INSERT OR REPLACE INTO price_points (symbol, ts, price_cents) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, p := range points {
		if _, err := stmt.ExecContext(ctx, p.Symbol, p.Timestamp, p.PriceCents); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (database *SQLiteDatabase) LatestPrices(ctx context.Context) (map[string]int64, error) {
	rows, err := database.db.QueryContext(ctx, `
		SELECT p.symbol, p.price_cents
		FROM price_points p
		INNER JOIN (
			SELECT symbol, MAX(ts) as max_ts
			FROM price_points
			GROUP BY symbol
		) latest ON p.symbol = latest.symbol AND p.ts = latest.max_ts
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make(map[string]int64)
	for rows.Next() {
		var sym string
		var price int64
		if err := rows.Scan(&sym, &price); err != nil {
			return nil, err
		}
		res[sym] = price
	}
	return res, rows.Err()
}

func (database *SQLiteDatabase) PriceHistory(ctx context.Context, symbol string, since time.Time) ([]PricePoint, error) {
	rows, err := database.db.QueryContext(
		ctx,
		`SELECT symbol, ts, price_cents FROM price_points WHERE symbol = ? AND ts >= ? ORDER BY ts ASC`,
		symbol,
		since.Unix(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []PricePoint
	for rows.Next() {
		var p PricePoint
		if err := rows.Scan(&p.Symbol, &p.Timestamp, &p.PriceCents); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	return points, rows.Err()
}

// NewMarket initializes companies, seeds 180 days of realistic stable historical data if absent,
// and starts the periodic price simulation engine.
func NewMarket(database Database) (*Market, error) {
	ctx := context.Background()
	existing, err := database.ListCompanies(ctx)
	if err != nil {
		return nil, err
	}

	compMap := make(map[string]Company)
	if len(existing) == 0 {
		// Insert initial companies into SQLite
		for _, c := range InitialCompanies {
			var compOf sql.NullString
			if c.ComplementaryOf != "" {
				compOf.String = c.ComplementaryOf
				compOf.Valid = true
			}
			if sdb, ok := database.(*SQLiteDatabase); ok {
				_, _ = sdb.db.Exec(
					`INSERT INTO companies (symbol, name, complementary_of, base_price_cents) VALUES (?, ?, ?, ?)`,
					c.Symbol,
					c.Name,
					compOf,
					c.BasePriceCents,
				)
			}
			compMap[c.Symbol] = c
		}
	} else {
		for _, c := range existing {
			compMap[c.Symbol] = c
		}
	}

	// Check if historical price points exist
	latestPrices, _ := database.LatestPrices(ctx)
	prices := make(map[string]int64)

	if len(latestPrices) == 0 {
		// Generate 180 days of stable historical data
		r := rand.New(rand.NewPCG(42, 108))
		now := time.Now().Unix()
		var points []PricePoint
		// Generate complementary shocks between VOLT and FUEL
		for day := 180; day >= 0; day-- {
			ts := now - int64(day*86400)
			commonShock := (r.Float64() - 0.49) * 0.04

			for _, c := range InitialCompanies {
				drift := (r.Float64() - 0.495) * 0.02
				if c.Symbol == "VOLT" {
					drift += commonShock
				} else if c.Symbol == "FUEL" {
					drift -= commonShock // opposite movement for complementary stock
				}

				// Price around base price with stable noise
				factor := 1.0 + 0.15*math.Sin(float64(day)/12.0) + drift
				p := int64(float64(c.BasePriceCents) * factor)
				if p < 100 {
					p = 100
				}
				points = append(points, PricePoint{
					Symbol:     c.Symbol,
					PriceCents: p,
					Timestamp:  ts,
				})
				if day == 0 {
					prices[c.Symbol] = p
				}
			}
		}
		_ = database.SavePricePoints(ctx, points)
	} else {
		for sym, c := range compMap {
			if p, ok := latestPrices[sym]; ok {
				prices[sym] = p
			} else {
				prices[sym] = c.BasePriceCents
			}
		}
	}

	m := &Market{
		database:  database,
		prices:    prices,
		companies: compMap,
		sentiment: make(map[string]float64),
		rng:       rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 99)),
	}

	go m.runTicker()

	return m, nil
}

func (m *Market) runTicker() {
	ticker := time.NewTicker(2 * time.Second)
	saveTicker := time.NewTicker(15 * time.Second)

	for {
		select {
		case <-ticker.C:
			m.mu.Lock()
			for sym, p := range m.prices {
				// baseline Brownian drift
				baseNoise := (m.rng.Float64() - 0.498) * 0.004
				sentimentPressure := m.sentiment[sym] * 0.015

				multiplier := 1.0 + baseNoise + sentimentPressure
				newP := int64(float64(p) * multiplier)
				if newP < 100 {
					newP = 100
				}
				m.prices[sym] = newP
			}
			m.mu.Unlock()

		case <-saveTicker.C:
			m.mu.RLock()
			now := time.Now().Unix()
			var points []PricePoint
			for sym, p := range m.prices {
				points = append(points, PricePoint{
					Symbol:     sym,
					PriceCents: p,
					Timestamp:  now,
				})
			}
			m.mu.RUnlock()
			_ = m.database.SavePricePoints(context.Background(), points)
		}
	}
}

func (m *Market) GetPrice(symbol string) (int64, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.prices[symbol]
	return p, ok
}

func (m *Market) GetAllPrices() map[string]int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make(map[string]int64, len(m.prices))
	for k, v := range m.prices {
		res[k] = v
	}
	return res
}

func (m *Market) GetCompany(symbol string) (Company, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.companies[symbol]
	return c, ok
}

// ApplyShock alters market price immediately by alpha, handling complementary inversion.
func (m *Market) ApplyShock(symbol string, alpha float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if symbol == "" || symbol == "ALL" {
		for sym, p := range m.prices {
			move := alpha
			c := m.companies[sym]
			if c.ComplementaryOf != "" && sym == "FUEL" {
				move = -alpha
			}
			newP := int64(float64(p) * (1.0 + move))
			if newP < 100 {
				newP = 100
			}
			m.prices[sym] = newP
		}
		return
	}

	p, ok := m.prices[symbol]
	if ok {
		newP := int64(float64(p) * (1.0 + alpha))
		if newP < 100 {
			newP = 100
		}
		m.prices[symbol] = newP
	}

	if c, ok := m.companies[symbol]; ok && c.ComplementaryOf != "" {
		if compP, ok2 := m.prices[c.ComplementaryOf]; ok2 {
			compNewP := int64(float64(compP) * (1.0 - alpha))
			if compNewP < 100 {
				compNewP = 100
			}
			m.prices[c.ComplementaryOf] = compNewP
		}
	}
}

// SetPrice allows directly overriding the price of a security (used by admin controls).
func (m *Market) SetPrice(symbol string, priceCents int64) bool {
	if priceCents < 100 {
		priceCents = 100
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.prices[symbol]; !ok {
		return false
	}
	m.prices[symbol] = priceCents
	return true
}
func (m *Market) SetSentiment(sentiment map[string]float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentiment = sentiment
}

// Handlers for /price and /price/history
func priceHandler(m *Market) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		symbol := r.URL.Query().Get("symbol")
		if symbol != "" {
			p, ok := m.GetPrice(symbol)
			if !ok {
				http.Error(w, "Unknown symbol", http.StatusNotFound)
				return
			}
			c, _ := m.GetCompany(symbol)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"symbol":           symbol,
				"name":             c.Name,
				"price_cents":      p,
				"price_formatted":  float64(p) / 100.0,
				"complementary_of": c.ComplementaryOf,
			})
			return
		}

		all := m.GetAllPrices()
		res := make([]map[string]any, 0, len(all))
		for sym, price := range all {
			c, _ := m.GetCompany(sym)
			res = append(res, map[string]any{
				"symbol":           sym,
				"name":             c.Name,
				"price_cents":      price,
				"price_formatted":  float64(price) / 100.0,
				"complementary_of": c.ComplementaryOf,
			})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(res)
	}
}

func historyHandler(database Database) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		symbol := r.URL.Query().Get("symbol")
		if symbol == "" {
			http.Error(w, "Query param 'symbol' is required", http.StatusBadRequest)
			return
		}

		days := 180
		if dStr := r.URL.Query().Get("days"); dStr != "" {
			if d, err := strconv.Atoi(dStr); err == nil && d > 0 {
				days = d
			}
		}
		since := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
		points, err := database.PriceHistory(r.Context(), symbol, since)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(points)
	}
}
