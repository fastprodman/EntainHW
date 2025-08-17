package e2etests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"
)

const (
	baseURL   = "http://localhost:8080"
	timeout   = 5 * time.Second
	waitReady = 20 * time.Second
)

var httpClient = &http.Client{Timeout: timeout}

func TestE2E_TransactionsFlow(t *testing.T) {
	waitUntilReady(t, 1) // wait until GET /user/1/balance works

	t.Run("user1_initial_balance_zero", func(t *testing.T) {
		got := getBalanceString(t, 1)
		want := "0.00"
		if got != want {
			t.Fatalf("initial balance mismatch: want %s, got %s", want, got)
		}
	})

	t.Run("user1_win_increases_balance", func(t *testing.T) {
		tid := uniqTxID("u1-win-10_15")
		code, body := postTransaction(t, 1, "game", "win", "10.15", tid)
		if code != http.StatusOK {
			t.Fatalf("win tx: want 200, got %d (%s)", code, body)
		}
		got := getBalanceString(t, 1)
		if got != "10.15" {
			t.Fatalf("after win: want 10.15, got %s", got)
		}
	})

	t.Run("user1_duplicate_transaction_conflict", func(t *testing.T) {
		tid := uniqTxID("u1-dup-5_00")
		// first time should pass
		code, body := postTransaction(t, 1, "game", "win", "5.00", tid)
		if code != http.StatusOK {
			t.Fatalf("first send: want 200, got %d (%s)", code, body)
		}
		// duplicate should NOT be applied (service returns 409 per your handler)
		code, body = postTransaction(t, 1, "game", "win", "5.00", tid)
		if code != http.StatusConflict {
			t.Fatalf("duplicate send: want 409, got %d (%s)", code, body)
		}
		// balance should be increased only once: 10.15 + 5.00 = 15.15
		got := getBalanceString(t, 1)
		if got != "15.15" {
			t.Fatalf("after duplicate: want 15.15, got %s", got)
		}
	})

	t.Run("user1_lose_decreases_balance", func(t *testing.T) {
		tid := uniqTxID("u1-lose-1_15")
		code, body := postTransaction(t, 1, "game", "lose", "1.15", tid)
		if code != http.StatusOK {
			t.Fatalf("lose tx: want 200, got %d (%s)", code, body)
		}
		// 15.15 - 1.15 = 14.00
		got := getBalanceString(t, 1)
		if got != "14.00" {
			t.Fatalf("after lose: want 14.00, got %s", got)
		}
	})
}

func TestE2E_InsufficientFundsAndValidation(t *testing.T) {
	waitUntilReady(t, 2)

	t.Run("user2_insufficient_funds_on_lose", func(t *testing.T) {
		// initial is 0.00
		got := getBalanceString(t, 2)
		if got != "0.00" {
			t.Fatalf("user2 initial: want 0.00, got %s", got)
		}

		tid := uniqTxID("u2-lose-1_00")
		code, body := postTransaction(t, 2, "game", "lose", "1.00", tid)
		if code != http.StatusConflict { // your handler maps insufficient funds to 409
			t.Fatalf("insufficient funds: want 409, got %d (%s)", code, body)
		}
		// balance unchanged
		got = getBalanceString(t, 2)
		if got != "0.00" {
			t.Fatalf("after insufficient: want 0.00, got %s", got)
		}
	})

	t.Run("user3_invalid_state", func(t *testing.T) {
		waitUntilReady(t, 3)
		tid := uniqTxID("u3-bad-state")
		code, _ := postTransaction(t, 3, "game", "invalid", "1.00", tid)
		if code != http.StatusBadRequest {
			t.Fatalf("bad state: want 400, got %d", code)
		}
	})

	t.Run("user3_invalid_amount_precision", func(t *testing.T) {
		tid := uniqTxID("u3-bad-amount")
		code, _ := postTransaction(t, 3, "game", "win", "1.234", tid)
		if code != http.StatusBadRequest {
			t.Fatalf("bad amount precision: want 400, got %d", code)
		}
	})

	t.Run("user3_invalid_source_type", func(t *testing.T) {
		tid := uniqTxID("u3-bad-source")
		code, _ := postTransactionWithHeader(t, 3, "bad-source", "win", "1.00", tid)
		if code != http.StatusBadRequest {
			t.Fatalf("bad source-type: want 400, got %d", code)
		}
	})
}

/* -------------------- helpers -------------------- */

func getBalanceString(t *testing.T, userID uint64) string {
	t.Helper()

	u := fmt.Sprintf("%s/user/%d/balance", baseURL, userID)

	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET %s: want 200, got %d (%s)", u, resp.StatusCode, string(b))
	}

	var payload struct {
		UserID  uint64 `json:"userId"`
		Balance string `json:"balance"`
	}

	err = json.NewDecoder(resp.Body).Decode(&payload)
	if err != nil {
		t.Fatalf("decode json: %v", err)
	}

	// quick sanity check
	if payload.UserID != userID {
		t.Fatalf("userId mismatch: want %d, got %d", userID, payload.UserID)
	}
	// ensure two-decimal format by parsing and reformatting
	if _, perr := parseMoney(payload.Balance); perr != nil {
		t.Fatalf("invalid balance format %q: %v", payload.Balance, perr)
	}

	return payload.Balance
}

func postTransaction(t *testing.T, userID uint64, source, state, amount, txid string) (int, string) {
	t.Helper()
	return postTransactionWithHeader(t, userID, source, state, amount, txid)
}

func postTransactionWithHeader(t *testing.T, userID uint64, source, state, amount, txid string) (int, string) {
	t.Helper()

	body := map[string]string{
		"state":         state,
		"amount":        amount,
		"transactionId": txid,
	}
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	u := fmt.Sprintf("%s/user/%d/transaction", baseURL, userID)
	req, err := http.NewRequest(http.MethodPost, u, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Source-Type", source)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

// waitUntilReady waits until GET /user/{userID}/balance responds 200 or times out.
func waitUntilReady(t *testing.T, userID uint64) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), waitReady)
	defer cancel()

	u := fmt.Sprintf("%s/user/%d/balance", baseURL, userID)

	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			t.Fatalf("service not ready at %s within %s", u, waitReady)
		case <-tick.C:
			req, _ := http.NewRequest(http.MethodGet, u, nil)
			resp, err := httpClient.Do(req)
			if err != nil {
				// if it's a dial error, keep waiting
				if isConnRefused(err) {
					continue
				}
				// any other network error -> keep waiting briefly
				continue
			}
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound {
				// OK if endpoint is up (even if user not found yet).
				return
			}
		}
	}
}

func uniqTxID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func parseMoney(s string) (int64, error) {
	// expect something like "12.34" -> 1234 cents
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	neg := false
	if s[0] == '-' {
		neg = true
		s = s[1:]
	}
	parts := bytes.Split([]byte(s), []byte{'.'})
	if len(parts) == 1 {
		parts = append(parts, []byte("00"))
	}
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid")
	}
	if len(parts[1]) != 2 {
		return 0, fmt.Errorf("need 2 decimals")
	}
	intPart, err := strconv.ParseInt(string(parts[0]), 10, 64)
	if err != nil {
		return 0, err
	}
	fracPart, err := strconv.ParseInt(string(parts[1]), 10, 64)
	if err != nil {
		return 0, err
	}
	cents := intPart*100 + fracPart
	if neg {
		cents = -cents
	}
	return cents, nil
}

func isConnRefused(err error) bool {
	var nerr net.Error
	if ok := errorAs(err, &nerr); ok {
		return true
	}
	// fallback: string match
	return stringsContains(err.Error(), "connection refused") ||
		stringsContains(err.Error(), "actively refused") ||
		stringsContains(err.Error(), "No connection could be made")
}

/* small stdlib-free helpers to keep imports tight */

func errorAs(err error, target any) bool { // simplified errors.As
	switch t := target.(type) {
	case *net.Error:
		var ne net.Error
		if ok := errorsAs(err, &ne); ok {
			*t = ne
			return true
		}
	}
	return false
}

func errorsAs(err error, target interface{}) bool {
	return false // we don't actually need full errors.As behavior here
}

func stringsContains(s, substr string) bool { return bytes.Contains([]byte(s), []byte(substr)) }
