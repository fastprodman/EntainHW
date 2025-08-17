package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/fastprodman/EntainHW/internal/repos/transactions"
	"github.com/fastprodman/EntainHW/internal/repos/users"
	"github.com/fastprodman/EntainHW/internal/services/balance"
	"github.com/go-chi/chi/v5"
)

// HandlerProvider wraps a BalanceService and exposes HTTP handlers.
type HandlerProvider struct {
	svc balance.BalanceService
}

// NewHandler returns a new Handler provider.
func NewHandler(svc balance.BalanceService) *HandlerProvider {
	return &HandlerProvider{svc: svc}
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	err := json.NewEncoder(w).Encode(v)
	if err != nil {
		// Log the error with slog
		slog.Error("failed to encode JSON response", "error", err)

		// As best-effort, write a minimal error payload if headers not sent
		http.Error(w, `{"error":"internal json encode failure"}`, http.StatusInternalServerError)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// parseUserIDFromPath reads `{userId}` from chi routes like:
//
//	GET  /user/{userId}/balance
//	POST /user/{userId}/transaction
func parseUserIDFromPath(r *http.Request) (uint64, error) {
	idStr := chi.URLParam(r, "userId")
	if idStr == "" {
		return 0, fmt.Errorf("missing userId")
	}

	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid userId: %w", err)
	}
	if id == 0 {
		return 0, fmt.Errorf("invalid userId: must be positive")
	}

	return id, nil
}

func parseSourceType(h http.Header) (balance.SourceType, error) {
	raw := strings.ToLower(strings.TrimSpace(h.Get("Source-Type")))
	switch raw {
	case "game":
		return balance.SourceGame, nil
	case "server":
		return balance.SourceServer, nil
	case "payment":
		return balance.SourcePayment, nil
	default:
		return "", fmt.Errorf("invalid Source-Type")
	}
}

type txRequest struct {
	State         string `json:"state"`
	Amount        string `json:"amount"`
	TransactionID string `json:"transactionId"`
}

func parseTxState(s string) (balance.TxState, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "win":
		return balance.TxWin, nil
	case "lose":
		return balance.TxLose, nil
	default:
		return "", fmt.Errorf("invalid state")
	}
}

// parseAmountCents converts a decimal string with up to 2 fractional digits into cents.
func parseAmountCents(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("amount required")
	}
	neg := false
	if s[0] == '+' {
		s = s[1:]
	}
	if s[0] == '-' {
		neg = true
		s = s[1:]
	}
	parts := strings.Split(s, ".")
	if len(parts) > 2 {
		return 0, fmt.Errorf("invalid amount")
	}
	intPart := parts[0]
	frac := "00"
	if len(parts) == 2 {
		if len(parts[1]) > 2 {
			return 0, fmt.Errorf("amount supports up to 2 decimals")
		}
		frac = parts[1] + strings.Repeat("0", 2-len(parts[1]))
	}
	ip, err := strconv.ParseInt(intPart, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid amount integer")
	}
	fp, err := strconv.ParseInt(frac, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid amount fractional")
	}
	total := ip*100 + fp
	if neg {
		total = -total
	}
	if total <= 0 {
		return 0, fmt.Errorf("amount must be > 0")
	}
	return total, nil
}

// --- Handlers ---

// GetBalanceHandler handles GET /user/{userId}/balance
func (h *HandlerProvider) GetBalanceHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUserIDFromPath(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid userId in path")
		return
	}

	bal, err := h.svc.GetBalance(r.Context(), userID)
	if err != nil {
		// domain mapping
		if errors.Is(err, users.ErrUserNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}

		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// spec: response has userId (uint64) and balance as string with 2 decimals
	resp := map[string]any{
		"userId":  userID,
		"balance": fmt.Sprintf("%.2f", float64(bal)/100.0),
	}
	writeJSON(w, http.StatusOK, resp)
}

// ProcessTransactionHandler handles POST /user/{userId}/transaction
func (h *HandlerProvider) ProcessTransactionHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUserIDFromPath(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid userId in path")
		return
	}

	source, err := parseSourceType(r.Header)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid Source-Type header")
		return
	}

	// Limit body size; disallow unknown fields
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB cap
	defer r.Body.Close()

	var req txRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	err = dec.Decode(&req)
	if err != nil {
		if errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "empty body")
			return
		}

		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	state, err := parseTxState(req.State)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid state")
		return
	}
	amountCents, err := parseAmountCents(req.Amount)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.TransactionID == "" {
		writeError(w, http.StatusBadRequest, "transactionId required")
		return
	}

	tx := balance.Transaction{
		TransactionID: req.TransactionID,
		UserID:        userID,
		Source:        source,
		State:         state,
		AmountMinor:   amountCents,
	}

	err = h.svc.ProcessTransaction(r.Context(), tx)
	if err != nil {
		switch {
		case errors.Is(err, transactions.ErrDuplicateTransaction):
			writeError(w, http.StatusConflict, "duplicate transaction")
			return
		case errors.Is(err, users.ErrInsufficientFunds):
			writeError(w, http.StatusConflict, "insufficient funds")
			return
		case errors.Is(err, users.ErrUserNotFound):
			writeError(w, http.StatusNotFound, "user not found")
			return
		default:
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}

	// spec says 200 OK on success (payload is up to you)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
