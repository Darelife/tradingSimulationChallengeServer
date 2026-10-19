# Trading Simulation Challenge Server

A real-time trading simulation backend with simulated companies, price Brownian motion, live news event shocks with panic/excitement virtual user reactions, order book execution, full suite of administrative controls, and token-bucket rate limiting.

---

## Admin Controls & Authentication

All protected endpoints and administrative controls require the user's `email` and `password` on every request. Pass them via:

1. **Inside the JSON body**:
   ```json
   {
     "email": "user@goa.bits-pilani.ac.in",
     "password": "yourpassword",
     "symbol": "VOLT",
     "quantity": 10
   }
   ```
2. **HTTP Basic Auth**:
   `-u "admin@domain.com:password"`
3. **Query parameters** (convenient for `GET` endpoints):
   `?email=admin@domain.com&password=password`

---

## Admin Endpoints

All `/admin/*` routes require credentials for an account with `is_admin = 1`.

### 1. Trigger a Random Event
Picks a random event from the 100-event catalog and applies the price shock immediately.
```bash
curl -X POST http://localhost:8080/admin/event \
  -H "Content-Type: application/json" \
  -d '{"email":"<admin_email>","password":"<admin_password>"}'
```

### 2. View All 100 Events in the Catalog
```bash
curl -u "<admin_email>:<admin_password>" http://localhost:8080/admin/events
```

### 3. Trigger a Specific Event by ID
```bash
curl -X POST http://localhost:8080/admin/event/trigger \
  -H "Content-Type: application/json" \
  -d '{"email":"<admin_email>","password":"<admin_password>","event_id":15}'
```

### 4. Override / Set Stock Price Directly
Overrides a security's live market price (value in paise: ₹1 = 100 paise).
```bash
curl -X POST http://localhost:8080/admin/price \
  -H "Content-Type: application/json" \
  -d '{"email":"<admin_email>","password":"<admin_password>","symbol":"VOLT","price_cents":95000}'
```

### 5. List All Users & Account Balances
```bash
curl -u "<admin_email>:<admin_password>" http://localhost:8080/admin/users
```

### 6. Adjust a User's Cash Balance
Sets a user's cash balance (in paise).
```bash
curl -X POST http://localhost:8080/admin/user/balance \
  -H "Content-Type: application/json" \
  -d '{"email":"<admin_email>","password":"<admin_password>","target_email":"trader@goa.bits-pilani.ac.in","balance_cents":5000000}'
```

### 7. Grant / Revoke Admin Role
```bash
curl -X POST http://localhost:8080/admin/user/role \
  -H "Content-Type: application/json" \
  -d '{"email":"<admin_email>","password":"<admin_password>","target_email":"trader@goa.bits-pilani.ac.in","is_admin":true}'
```

### 8. View Complete Global Order Log
```bash
curl -u "<admin_email>:<admin_password>" "http://localhost:8080/admin/orders?limit=50"
```

---

## Market & Prices (Public)

Six companies with 180 days of historical seed data:
- `NEXA`: Nexatech Cloud Systems
- `ORGA`: Organica Green Foods
- `VOLT`: Voltmobility Electric (Complementary of `FUEL`)
- `FUEL`: Fuelcore Petrochemicals (Complementary of `VOLT`)
- `MEDC`: Medcure Pharmaceuticals
- `AERO`: Aerosky Logistics

### Get Current Prices
```bash
# All securities
curl http://localhost:8080/price

# Single security
curl "http://localhost:8080/price?symbol=VOLT"
```

### Price History
```bash
curl "http://localhost:8080/price/history?symbol=VOLT&days=180"
```

### Active Event Description
```bash
curl http://localhost:8080/event
```

---

## User Registration & Password Reset

### Register
```bash
curl -X POST http://localhost:8080/register/start \
  -H "Content-Type: application/json" \
  -d '{"email":"f20220001@goa.bits-pilani.ac.in"}'

curl -X POST http://localhost:8080/register/complete \
  -H "Content-Type: application/json" \
  -d '{"challenge_id":"<id>","code":"123456","password":"secretpassword"}'
```

### Forgot Password
```bash
curl -X POST http://localhost:8080/forgot-password/start \
  -H "Content-Type: application/json" \
  -d '{"email":"f20220001@goa.bits-pilani.ac.in"}'

curl -X POST http://localhost:8080/forgot-password/complete \
  -H "Content-Type: application/json" \
  -d '{"challenge_id":"<id>","code":"<code>","password":"newpassword"}'
```

---

## Trading & Portfolio (Requires Email & Password)

### Buy & Sell
```bash
# Buy shares (credentials in JSON payload)
curl -X POST http://localhost:8080/buy \
  -H "Content-Type: application/json" \
  -d '{"email":"f20220001@goa.bits-pilani.ac.in","password":"secretpassword","symbol":"VOLT","quantity":10}'

# Sell shares
curl -X POST http://localhost:8080/sell \
  -H "Content-Type: application/json" \
  -d '{"email":"f20220001@goa.bits-pilani.ac.in","password":"secretpassword","symbol":"VOLT","quantity":5}'
```

### List Orders
```bash
curl -u "f20220001@goa.bits-pilani.ac.in:secretpassword" http://localhost:8080/orders
```

### Portfolio & Balance
```bash
curl -u "f20220001@goa.bits-pilani.ac.in:secretpassword" http://localhost:8080/portfolio
```

---

## Rate Limiting
Built-in token-bucket rate limiter per IP per route:
- Registration / password start: 2 req/s (burst 5)
- Registration complete: 5 req/s (burst 10)
- Trading & orders: 30 req/s (burst 60)
- Price feeds: 60 req/s (burst 120)
- Admin controls: 5–10 req/s (burst 10–20)
