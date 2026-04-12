package payments

import (
	"fmt"
	"strconv"
	"strings"
)

type Product struct {
	Key      string
	Title    string
	Coins    int64
	Amount   int64
	Currency string
}

func ProductWithCoins(key, title string, coins, priceRub int64) Product {
	return Product{
		Key:      strings.TrimSpace(key),
		Title:    strings.TrimSpace(title),
		Coins:    coins,
		Amount:   priceRub,
		Currency: "RUB",
	}
}

func CoinProduct(key, title string, priceRub int64) (Product, error) {
	coins := inferCoins(key, title)
	if coins <= 0 {
		return Product{}, fmt.Errorf("cannot infer coins from key=%q title=%q", key, title)
	}
	return ProductWithCoins(key, title, coins, priceRub), nil
}

func MustCoinProduct(key, title string, priceRub int64) Product {
	product, err := CoinProduct(key, title, priceRub)
	if err != nil {
		panic(err)
	}
	return product
}

func inferCoins(values ...string) int64 {
	for _, value := range values {
		raw := strings.TrimSpace(value)
		if raw == "" {
			continue
		}

		var digits strings.Builder
		for _, ch := range raw {
			if ch >= '0' && ch <= '9' {
				digits.WriteRune(ch)
				continue
			}
			if digits.Len() > 0 {
				break
			}
		}

		if digits.Len() == 0 {
			continue
		}
		n, err := strconv.ParseInt(digits.String(), 10, 64)
		if err == nil && n > 0 {
			return n
		}
	}
	return 0
}
