package payments

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/elum-bots/core/internal/db"
)

type fakeGateway struct{}

func (fakeGateway) CreateTransaction(_ context.Context, _ CreateRequest) (CreateResult, error) {
	return CreateResult{
		TransactionID: "tx-1",
		RedirectURL:   "https://pay.example/tx-1",
		Status:        "pending",
	}, nil
}

func TestServiceCreateAndConfirm(t *testing.T) {
	t.Setenv("ONBOARDING_BONUS", "1")

	store, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "bot.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	if _, err := store.Users.Ensure(context.Background(), "u1"); err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}

	svc, err := NewService(store, Config{
		Enabled:       true,
		PaymentMethod: 2,
		Products: []Product{
			MustCoinProduct("10_coins", "10 монет", 100),
		},
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	svc.SetGateway(fakeGateway{})

	tx, err := svc.CreateLink(context.Background(), "u1", "u1", "10_coins")
	if err != nil {
		t.Fatalf("CreateLink() error = %v", err)
	}
	if got, want := tx.ProductKey, "10_coins"; got != want {
		t.Fatalf("product key = %q, want %q", got, want)
	}

	updated, rewardedNow, err := svc.HandleCallback(context.Background(), Callback{
		ID:            tx.TransactionID,
		Status:        "CONFIRMED",
		Currency:      "RUB",
		PaymentMethod: 2,
	})
	if err != nil {
		t.Fatalf("HandleCallback() error = %v", err)
	}
	if !rewardedNow {
		t.Fatalf("rewardedNow = false, want true")
	}
	if got, want := updated.Status, "confirmed"; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}

	user, err := store.Users.Get(context.Background(), "u1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got, want := user.Coins, int64(11); got != want {
		t.Fatalf("coins = %d, want %d", got, want)
	}
}
