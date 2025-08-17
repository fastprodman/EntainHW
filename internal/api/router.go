package api

import (
	"net/http"

	"github.com/fastprodman/EntainHW/internal/services/balance"
	"github.com/go-chi/chi/v5"
)

// NewRouter constructs an http.ServeMux with all API endpoints registered.
func NewRouter(svc balance.BalanceService) http.Handler {
	h := NewHandler(svc)
	r := chi.NewRouter()

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// These call your existing handlers; they still read from r.URL.Path,
	// which will be /user/{id}/balance etc. You could also refactor them
	// to read chi.URLParam(r, "userId") if you prefer.
	r.Get("/user/{userId}/balance", h.GetBalanceHandler)
	r.Post("/user/{userId}/transaction", h.ProcessTransactionHandler)

	return r
}
