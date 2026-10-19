package main

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"net/http"
	"sync"
	"time"
)

// MarketEvent holds description and market impact parameters.
type MarketEvent struct {
	ID          int     `json:"id"`
	Description string  `json:"description"`
	Symbol      string  `json:"symbol"` // Affected symbol or "ALL" for broader market
	Alpha       float64 `json:"alpha"`  // Fractional magnitude and direction
}

// 100 realistic simulation market events
var CatalogEvents = generate100Events()

func generate100Events() []MarketEvent {
	templates := []struct {
		desc   string
		symbol string
		alpha  float64
	}{
		// NEXA (Tech / Cloud)
		{"NEXA unveils breakthrough generative quantum computing API", "NEXA", 0.09},
		{"NEXA suffers critical 14-hour cloud outage affecting major banking clients", "NEXA", -0.08},
		{"NEXA signs multi-billion dollar enterprise contract with defense contractor", "NEXA", 0.07},
		{"Regulators initiate antitrust investigation into NEXA cloud monopolies", "NEXA", -0.06},
		{"NEXA quarterly operating margins beat consensus estimates by 320 bps", "NEXA", 0.05},
		{"Lead architect and key AI leadership resign unexpectedly from NEXA", "NEXA", -0.07},
		{"NEXA announces massive $500M open-market share buyback program", "NEXA", 0.04},
		{"Semiconductor supply bottleneck forces NEXA data center delivery delays", "NEXA", -0.05},
		{"NEXA patents high-efficiency tensor accelerator reducing power draw by 40%", "NEXA", 0.06},
		{"NEXA lowers full-year enterprise software guidance due to weak renewals", "NEXA", -0.05},

		// ORGA (Agriculture / Foods)
		{"Unprecedented drought drastically shrinks domestic organic grain harvest", "ORGA", -0.08},
		{"Bumper harvest and automated hydroponic facility triple ORGA output yields", "ORGA", 0.08},
		{"Government expands subsidized organic farming incentives nationwide", "ORGA", 0.06},
		{"Salmonella outbreak traced to third-party contract packaging facility", "ORGA", -0.09},
		{"ORGA closes nationwide exclusive distribution pact with premier supermarket chain", "ORGA", 0.07},
		{"Fertilizer and raw packaging tariffs surge input costs across agricultural sector", "ORGA", -0.05},
		{"ORGA introduces lab-grown sustainable protein line achieving viral consumer demand", "ORGA", 0.09},
		{"International fair-trade certification renewed with top ESG tier rating for ORGA", "ORGA", 0.03},
		{"Severe flash floods inundate primary regional greenhouse complexes", "ORGA", -0.07},
		{"Consumer index reports 24% annual growth in premium organic food spending", "ORGA", 0.04},

		// VOLT vs FUEL (Complementary pair)
		{"Federal mandates require 50% EV fleet adoption; massive tax credit enacted", "VOLT", 0.11},
		{"Major breakthrough in solid-state lithium battery doubles vehicle range", "VOLT", 0.10},
		{"Subsidies for electric vehicle buyers abruptly phased out in key provincial markets", "VOLT", -0.09},
		{"Cobalt and rare-earth export curbs strangle global battery manufacturing lines", "VOLT", -0.08},
		{"VOLT rolls out ultra-fast 10-minute commercial charging network nationwide", "VOLT", 0.07},
		{"OPEC announces surprise production cut of 2 million barrels daily", "FUEL", 0.10},
		{"Geopolitical tensions escalate near crucial maritime oil transit choke points", "FUEL", 0.09},
		{"Discovery of massive offshore deep-water crude reserves floods prospective supply", "FUEL", -0.10},
		{"Carbon emission taxes doubled on heavy fossil fuel extractors and refiners", "FUEL", -0.08},
		{"Global refinery fire causes acute regional diesel and aviation gasoline shortage", "FUEL", 0.08},
		{"VOLT battery recalls surge following multiple battery thermal runaway incidents", "VOLT", -0.10},
		{"FUEL announces deep transition investment into hydrogen fuel cell distribution", "FUEL", 0.06},
		{"International clean energy accord ratifies stringent combustion phaseout target", "VOLT", 0.08},
		{"Crude futures slide 8% as global industrial manufacturing indicators slow", "FUEL", -0.07},
		{"National grid upgrades incentivize dynamic vehicle-to-grid power purchase from VOLT", "VOLT", 0.05},

		// MEDC (Pharma)
		{"MEDC receives accelerated FDA approval for Phase-3 oncology immunotherapy", "MEDC", 0.12},
		{"Phase-3 clinical trials for MEDC flagship autoimmune compound fail endpoints", "MEDC", -0.13},
		{"Patent expiration lawsuit invalidates exclusivity on top-selling MEDC drug", "MEDC", -0.10},
		{"Global respiratory epidemic creates extraordinary demand for MEDC antivirals", "MEDC", 0.11},
		{"Price-cap legislation enacted on prescription pharmaceuticals", "MEDC", -0.08},
		{"MEDC acquires revolutionary CRISPR gene-editing startup", "MEDC", 0.07},
		{"Manufacturing contamination triggers nationwide recall of MEDC injectable antibiotics", "MEDC", -0.09},
		{"World Health Organization awards large procurement contract to MEDC", "MEDC", 0.06},
		{"MEDC reports double-digit royalty revenue growth from international licensing", "MEDC", 0.04},
		{"Class-action liability lawsuit filed over legacy pediatric medication side-effects", "MEDC", -0.06},

		// AERO (Airlines / Logistics)
		{"Global travel demand rebounds to historic record highs for summer holiday season", "AERO", 0.09},
		{"Air traffic controllers strike forces cancellation of 65% of domestic flights", "AERO", -0.10},
		{"AERO modernizes fleet with next-generation ultra-fuel-efficient turbofans", "AERO", 0.06},
		{"Volcanic ash cloud grounds transatlantic and transcontinental flight corridors", "AERO", -0.08},
		{"AERO secures profitable sovereign military logistics transport contract", "AERO", 0.07},
		{"Severe pilot and aircraft technician labor shortages trigger wage inflation", "AERO", -0.05},
		{"AERO expands into high-margin express pharmaceutical cold-chain cargo routes", "AERO", 0.05},
		{"Severe winter storm gridlocks busiest international connecting hub airports", "AERO", -0.06},
		{"Tourism board partners with AERO for subsidized international promotion campaign", "AERO", 0.04},
		{"Mechanical grounding order issued across widespread regional regional jet models", "AERO", -0.07},

		// Macro / Broad Market
		{"Central Bank unexpectedly cuts benchmark interest rates by 50 basis points", "ALL", 0.06},
		{"Central Bank issues hawkish forecast and hikes policy rates by 75 basis points", "ALL", -0.06},
		{"Consumer Confidence Index reaches highest level in five years", "ALL", 0.05},
		{"Headline inflation accelerates, eroding consumer purchasing power", "ALL", -0.05},
		{"National GDP annualized growth reading prints at robust 4.8%", "ALL", 0.06},
		{"Credit rating agency downgrades sovereign debt outlook to negative", "ALL", -0.07},
		{"Comprehensive corporate tax cuts passed through emergency legislative session", "ALL", 0.05},
		{"Foreign institutional investors stage massive single-day capital flight", "ALL", -0.06},
		{"Retail participation reaches all-time high with record domestic brokerage inflows", "ALL", 0.04},
		{"Currency experiences sharp currency depreciation raising imported input costs", "ALL", -0.04},
	}

	var events []MarketEvent
	id := 1

	for _, t := range templates {
		events = append(events, MarketEvent{
			ID:          id,
			Description: t.desc,
			Symbol:      t.symbol,
			Alpha:       t.alpha,
		})
		id++
	}

	// Procedurally expand variants up to 100
	modifiers := []struct {
		prefix string
		factor float64
	}{
		{"Breaking News: Unconfirmed reports suggest ", 0.7},
		{"Analyst Consensus: Institutional research notes ", 0.85},
		{"Government Bulletin: Regulators officially confirm ", 1.15},
		{"Industry Leak: Confidential whistleblowers reveal ", 1.05},
	}

	syms := []string{"NEXA", "ORGA", "VOLT", "FUEL", "MEDC", "AERO"}
	modIdx := 0

	for len(events) < 100 {
		mod := modifiers[modIdx%len(modifiers)]
		sym := syms[len(events)%len(syms)]
		direction := 1.0
		if len(events)%2 == 1 {
			direction = -1.0
		}
		alpha := direction * (0.03 + float64(len(events)%6)*0.012) * mod.factor
		alpha = math.Round(alpha*1000) / 1000

		desc := fmt.Sprintf("%s%s unexpected supply chain and strategic shift impacting market sentiment", mod.prefix, sym)
		events = append(events, MarketEvent{
			ID:          id,
			Description: desc,
			Symbol:      sym,
			Alpha:       alpha,
		})
		id++
		modIdx++
	}

	return events
}

// ActiveEventState tracks the currently broadcasted event.
type ActiveEventState struct {
	Event     *MarketEvent `json:"event"`
	StartedAt int64        `json:"started_at"`
	EndsAt    int64        `json:"ends_at"`
}

// EventEngine manages active events and virtual user order sentiment.
type EventEngine struct {
	market      *Market
	mu          sync.RWMutex
	active      *ActiveEventState
	vusers      *VirtualTraders
	lastEventID int
}
// VirtualTraders simulates ~50 automated agents reacting to news with panic/excitement.
type VirtualTraders struct {
	engine     *EventEngine
	market     *Market
	traderBias []float64 // sentiment sensitivity per virtual agent
}

func NewEventEngine(m *Market) *EventEngine {
	ee := &EventEngine{
		market: m,
	}

	vt := &VirtualTraders{
		engine: ee,
		market: m,
	}
	r := rand.New(rand.NewPCG(1234, 5678))
	for range 50 {
		// Each virtual trader has different sensitivity: 0.5 to 1.5
		vt.traderBias = append(vt.traderBias, 0.5+r.Float64())
	}
	ee.vusers = vt

	go vt.run()

	return ee
}

// TriggerRandomEvent is triggered by the admin command.
func (ee *EventEngine) TriggerRandomEvent() MarketEvent {
	ee.mu.Lock()
	defer ee.mu.Unlock()

	idx := rand.IntN(len(CatalogEvents))
	if CatalogEvents[idx].ID == ee.lastEventID && len(CatalogEvents) > 1 {
		idx = (idx + 1) % len(CatalogEvents)
	}

	selected := CatalogEvents[idx]
	ee.lastEventID = selected.ID

	now := time.Now().Unix()
	durationSeconds := int64(90) // 90 seconds impact duration

	ee.active = &ActiveEventState{
		Event:     &selected,
		StartedAt: now,
		EndsAt:    now + durationSeconds,
	}

	// Immediately apply alpha shock to prices
	ee.market.ApplyShock(selected.Symbol, selected.Alpha)

	return selected
}

// TriggerSpecificEvent fires a specific event by ID.
func (ee *EventEngine) TriggerSpecificEvent(id int) (*MarketEvent, error) {
	ee.mu.Lock()
	defer ee.mu.Unlock()

	var found *MarketEvent
	for _, ev := range CatalogEvents {
		if ev.ID == id {
			found = &ev
			break
		}
	}
	if found == nil {
		return nil, fmt.Errorf("event ID %d not found", id)
	}

	ee.lastEventID = found.ID
	now := time.Now().Unix()
	durationSeconds := int64(90)

	ee.active = &ActiveEventState{
		Event:     found,
		StartedAt: now,
		EndsAt:    now + durationSeconds,
	}

	ee.market.ApplyShock(found.Symbol, found.Alpha)
	return found, nil
}

func (ee *EventEngine) GetActive() *ActiveEventState {
	ee.mu.RLock()
	defer ee.mu.RUnlock()

	if ee.active == nil {
		return nil
	}
	if time.Now().Unix() > ee.active.EndsAt {
		return nil
	}
	return ee.active
}

// run executes virtual user trades and panic/excitement sentiment.
func (vt *VirtualTraders) run() {
	ticker := time.NewTicker(1 * time.Second)

	for range ticker.C {
		active := vt.engine.GetActive()

		sentiment := make(map[string]float64)

		if active != nil && active.Event != nil {
			ev := active.Event
			now := time.Now().Unix()
			remaining := float64(active.EndsAt - now)
			decay := math.Max(0.0, remaining/90.0)

			// Aggregate panic or excitement across 50 virtual users
			var aggregatePressure float64
			for _, bias := range vt.traderBias {
				noise := (rand.Float64() - 0.45) * 0.2
				individualReaction := ev.Alpha * bias * (1.0 + noise)
				aggregatePressure += individualReaction
			}
			aggregatePressure = (aggregatePressure / float64(len(vt.traderBias))) * decay

			if ev.Symbol == "ALL" {
				for _, c := range InitialCompanies {
					sentiment[c.Symbol] = aggregatePressure
				}
			} else {
				sentiment[ev.Symbol] = aggregatePressure
				// Complementary virtual user reaction
				c, ok := vt.market.GetCompany(ev.Symbol)
				if ok && c.ComplementaryOf != "" {
					sentiment[c.ComplementaryOf] = -aggregatePressure
				}
			}
		} else {
			// Ambient low-volume retail sentiment noise
			for _, c := range InitialCompanies {
				sentiment[c.Symbol] = (rand.Float64() - 0.5) * 0.05
			}
		}

		vt.market.SetSentiment(sentiment)
	}
}

// Handlers for /event and admin /admin/event
func eventHandler(ee *EventEngine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		active := ee.GetActive()
		w.Header().Set("Content-Type", "application/json")

		if active == nil {
			json.NewEncoder(w).Encode(map[string]any{
				"active":      false,
				"description": "No active event. Markets are trading on normal ambient conditions.",
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]any{
			"active":      true,
			"event_id":    active.Event.ID,
			"description": active.Event.Description,
			"symbol":      active.Event.Symbol,
			"alpha":       active.Event.Alpha,
			"started_at":  active.StartedAt,
			"ends_at":     active.EndsAt,
		})
	}
}

func adminTriggerEventHandler(ee *EventEngine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ev := ee.TriggerRandomEvent()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"message": "Event triggered successfully",
			"event":   ev,
		})
	}
}
