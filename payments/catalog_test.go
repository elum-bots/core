package payments

import "testing"

func TestCoinProduct(t *testing.T) {
	product, err := CoinProduct("10_coins", "10 монет", 100)
	if err != nil {
		t.Fatalf("CoinProduct() error = %v", err)
	}
	if got, want := product.Key, "10_coins"; got != want {
		t.Fatalf("key = %q, want %q", got, want)
	}
	if got, want := product.Coins, int64(10); got != want {
		t.Fatalf("coins = %d, want %d", got, want)
	}
	if got, want := product.Amount, int64(100); got != want {
		t.Fatalf("amount = %d, want %d", got, want)
	}
}
