package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/fastprodman/EntainHW/internal/services/balance"
)

// NewServer creates and returns a configured *http.Server for the balance API.
func NewServer(port uint16, svc balance.BalanceService) *http.Server {
	mux := NewRouter(svc)

	addr := fmt.Sprintf(":%d", port)

	return &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}
}
