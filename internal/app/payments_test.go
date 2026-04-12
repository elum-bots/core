package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	elumbot "github.com/elum-bots/core/internal/bot"
	"github.com/elum-bots/core/internal/db"
	"github.com/elum-bots/core/payments"
)

type testGateway struct{}

func (testGateway) CreateTransaction(context.Context, payments.CreateRequest) (payments.CreateResult, error) {
	return payments.CreateResult{}, nil
}

func TestRuntimeHTTPHandlerCallsPaymentSuccessHook(t *testing.T) {
	store, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "bot.sqlite"))
	if err != nil {
		t.Fatalf("db.Open() error = %v", err)
	}
	defer store.Close()

	if _, err := store.Users.Ensure(context.Background(), "u1"); err != nil {
		t.Fatalf("Users.Ensure() error = %v", err)
	}
	if _, err := store.Payments.CreateTransaction(context.Background(), db.PaymentTransaction{
		TransactionID:  "tx-1",
		UserID:         "u1",
		PlatformUserID: "777",
		ProductKey:     "10_coins",
		ProductTitle:   "10 монет",
		Coins:          10,
		Amount:         100,
		Currency:       "RUB",
		Status:         "pending",
	}); err != nil {
		t.Fatalf("Payments.CreateTransaction() error = %v", err)
	}

	svc, err := payments.NewService(store, payments.Config{
		Enabled:  true,
		Products: []payments.Product{payments.ProductWithCoins("10_coins", "10 монет", 10, 100)},
	})
	if err != nil {
		t.Fatalf("payments.NewService() error = %v", err)
	}
	svc.SetGateway(testGateway{})

	called := false
	handler := runtimeHTTPHandler(
		Config{},
		svc,
		nil,
		nil,
		Dependencies{Store: store, Payments: svc},
		options{
			paymentHooks: []PaymentSuccessHook{
				func(_ context.Context, _ *elumbot.Bot, _ Dependencies, tx db.PaymentTransaction) error {
					called = tx.TransactionID == "tx-1"
					return nil
				},
			},
		},
	)

	body, _ := json.Marshal(map[string]any{
		"id":            "tx-1",
		"amount":        100,
		"currency":      "RUB",
		"status":        "CONFIRMED",
		"paymentMethod": 2,
	})
	req := httptest.NewRequest(http.MethodPost, "/payments/platega/callback", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if !called {
		t.Fatalf("payment success hook was not called")
	}

	user, err := store.Users.Get(context.Background(), "u1")
	if err != nil {
		t.Fatalf("Users.Get() error = %v", err)
	}
	if got, want := user.Coins, int64(11); got != want {
		t.Fatalf("user coins = %d, want %d", got, want)
	}
}
