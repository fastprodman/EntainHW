# Entain Test Task – Balance Service (Go + Postgres)

A small HTTP service that processes **win/lose** transactions from third-party providers and maintains per-user balances (in cents). Built with Go, Postgres, and Docker Compose.

**Time budget:** 12 hours

---

## Quick start

1. **Start the stack (Postgres + API):**

```bash
docker compose up -d
```

* The service starts in **DEV** mode by default (from `.env.dev`), and **pre-seeds users `1`, `2`, and `3`** with zero balance.
* Base URL: **[http://localhost:8080](http://localhost:8080)**

2. **Run all tests (unit + integration + e2e):**

```bash
go test ./...
```

> Make sure the stack is already running (`docker compose up -d`) before running tests, so the e2e suite can reach the API on `localhost:8080`.

---

## Endpoints

### Get balance

`GET /user/{userId}/balance`

**Response (200 OK):**

```json
{
  "userId": 1,
  "balance": "9.25"   // string with 2 decimals
}
```

**Errors:**

* `404 Not Found` — user does not exist
* `500 Internal Server Error` — unexpected error

---

### Process transaction

`POST /user/{userId}/transaction`

**Headers**

```
Source-Type: game | server | payment
Content-Type: application/json
```

**Body**

```json
{
  "state": "win",                // or "lose"
  "amount": "10.15",             // string, up to 2 decimals
  "transactionId": "unique-id"   // idempotency key
}
```

**Behavior**

* `state = "win"` → increases balance
* `state = "lose"` → decreases balance (never below 0)
* **Idempotent** by `transactionId`: the same ID is processed only once.

**Success**

* `200 OK`

**Errors**

* `409 Conflict` — duplicate `transactionId` or insufficient funds
* `400 Bad Request` — invalid header/body
* `404 Not Found` — user not found
* `500 Internal Server Error` — unexpected error

---

## Configuration

* The service reads environment from **`.env.dev`** by default (used by Docker Compose).
* It starts in **DEV** environment and **seeds users `1`, `2`, `3`**.

To **run without seed users** or in any non-DEV mode, change:

```
APP_ENV=DEV
```

…to any other value (e.g. `APP_ENV=PROD`) in `.env.dev`, then restart:

```bash
docker compose down -v
docker compose up -d
```

**HTTP base URL:** `http://localhost:8080`

---

## Example usage (curl)

```bash
# Get balance
curl -s http://localhost:8080/user/1/balance

# Win +10.15
curl -s -X POST "http://localhost:8080/user/1/transaction" \
  -H "Source-Type: game" -H "Content-Type: application/json" \
  -d '{"state":"win","amount":"10.15","transactionId":"tx-001"}'

# Lose -1.15
curl -s -X POST "http://localhost:8080/user/1/transaction" \
  -H "Source-Type: game" -H "Content-Type: application/json" \
  -d '{"state":"lose","amount":"1.15","transactionId":"tx-002"}'
```

---

## Project notes

* Balances are stored in **minor units (cents)** as integers to avoid floating point issues.
* Per-request idempotency is enforced by a unique constraint on `transaction_id`.
* Balance never goes negative (guarded at the DB level and in the service).

---

## Possible improvements

* **Add mocks** for services/repos and expand unit tests (e.g., using **mockery** to generate interfaces/mocks).
* **Metrics and traces** OpenTelemetry metrics and tracing could be added.
* **Provide a Go client library** for this API (typed requests/responses).

---

## Housekeeping

* Stop the stack:

```bash
docker compose down -v
```
