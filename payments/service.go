package payments

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/elum-bots/core/internal/db"
)

type Config struct {
	Enabled       bool
	PaymentMethod int
	ReturnURL     string
	FailedURL     string
	Products      []Product
}

type CreateRequest struct {
	Amount        int64
	Currency      string
	PaymentMethod int
	Description   string
	ReturnURL     string
	FailedURL     string
	Payload       string
}

type CreateResult struct {
	TransactionID string
	RedirectURL   string
	Status        string
}

type Callback struct {
	ID            string
	Amount        float64
	Currency      string
	Status        string
	PaymentMethod int
}

type Gateway interface {
	CreateTransaction(ctx context.Context, in CreateRequest) (CreateResult, error)
}

type Service struct {
	store      *db.Store
	gateway    Gateway
	enabled    bool
	method     int
	returnURL  string
	failedURL  string
	products   []Product
	productsBy map[string]Product
}

func NewService(store *db.Store, cfg Config) (*Service, error) {
	if store == nil {
		return nil, errors.New("store is nil")
	}

	products, productsBy, err := normalizeProducts(cfg.Products)
	if err != nil {
		return nil, err
	}

	return &Service{
		store:      store,
		enabled:    cfg.Enabled,
		method:     cfg.PaymentMethod,
		returnURL:  strings.TrimSpace(cfg.ReturnURL),
		failedURL:  strings.TrimSpace(cfg.FailedURL),
		products:   products,
		productsBy: productsBy,
	}, nil
}

func (s *Service) SetGateway(gw Gateway) {
	s.gateway = gw
}

func (s *Service) IsEnabled() bool {
	return s != nil && s.enabled && s.gateway != nil && len(s.products) > 0
}

func (s *Service) Products() []Product {
	if s == nil {
		return nil
	}
	out := make([]Product, len(s.products))
	copy(out, s.products)
	return out
}

func (s *Service) Product(key string) (Product, bool) {
	if s == nil {
		return Product{}, false
	}
	product, ok := s.productsBy[normalizeKey(key)]
	return product, ok
}

func (s *Service) CreateLink(ctx context.Context, userID, platformUserID, productKey string) (db.PaymentTransaction, error) {
	if !s.IsEnabled() {
		return db.PaymentTransaction{}, errors.New("payments disabled")
	}

	product, ok := s.Product(productKey)
	if !ok {
		return db.PaymentTransaction{}, errors.New("unknown product")
	}

	returnURL, err := normalizeOptionalURL(s.returnURL)
	if err != nil {
		return db.PaymentTransaction{}, fmt.Errorf("invalid PLATEGA_RETURN_URL: %w", err)
	}
	failedURL, err := normalizeOptionalURL(s.failedURL)
	if err != nil {
		return db.PaymentTransaction{}, fmt.Errorf("invalid PLATEGA_FAILED_URL: %w", err)
	}

	result, err := s.gateway.CreateTransaction(ctx, CreateRequest{
		Amount:        product.Amount,
		Currency:      product.Currency,
		PaymentMethod: s.method,
		Description:   product.Title,
		ReturnURL:     returnURL,
		FailedURL:     failedURL,
		Payload:       fmt.Sprintf("user=%s;product=%s", strings.TrimSpace(userID), product.Key),
	})
	if err != nil {
		return db.PaymentTransaction{}, err
	}

	tx := db.PaymentTransaction{
		TransactionID:  strings.TrimSpace(result.TransactionID),
		UserID:         strings.TrimSpace(userID),
		PlatformUserID: strings.TrimSpace(platformUserID),
		ProductKey:     product.Key,
		ProductTitle:   product.Title,
		Coins:          product.Coins,
		Amount:         product.Amount,
		Currency:       product.Currency,
		PaymentMethod:  int64(s.method),
		Status:         "pending",
		RedirectURL:    strings.TrimSpace(result.RedirectURL),
	}
	return s.store.Payments.CreateTransaction(ctx, tx)
}

func (s *Service) HandleCallback(ctx context.Context, cb Callback) (db.PaymentTransaction, bool, error) {
	if s == nil {
		return db.PaymentTransaction{}, false, errors.New("payments service is nil")
	}

	txID := strings.TrimSpace(cb.ID)
	if txID == "" {
		return db.PaymentTransaction{}, false, errors.New("empty transaction id")
	}

	tx, err := s.store.Payments.GetByTransactionID(ctx, txID)
	if err != nil {
		return db.PaymentTransaction{}, false, err
	}

	status := strings.ToLower(strings.TrimSpace(cb.Status))
	switch status {
	case "confirmed", "canceled":
	default:
		return db.PaymentTransaction{}, false, errors.New("invalid callback status")
	}

	var paidAt *time.Time
	if status == "confirmed" {
		now := time.Now().UTC()
		paidAt = &now
	}

	if err := s.store.Payments.MarkStatus(ctx, tx.TransactionID, status, int64(cb.PaymentMethod), paidAt); err != nil {
		return db.PaymentTransaction{}, false, err
	}

	rewardedNow := false
	if status == "confirmed" && !tx.Rewarded {
		if err := s.store.Users.AddCoins(ctx, tx.UserID, tx.Coins); err != nil {
			return db.PaymentTransaction{}, false, err
		}
		if err := s.store.Payments.MarkRewarded(ctx, tx.TransactionID); err != nil {
			return db.PaymentTransaction{}, false, err
		}
		tx.Rewarded = true
		rewardedNow = true
	}

	tx.Status = status
	tx.PaymentMethod = int64(cb.PaymentMethod)
	tx.PaidAt = paidAt
	return tx, rewardedNow, nil
}

func normalizeProducts(products []Product) ([]Product, map[string]Product, error) {
	out := make([]Product, 0, len(products))
	outByKey := make(map[string]Product, len(products))
	for _, product := range products {
		product.Key = normalizeKey(product.Key)
		product.Title = strings.TrimSpace(product.Title)
		product.Currency = strings.ToUpper(strings.TrimSpace(product.Currency))
		if product.Currency == "" {
			product.Currency = "RUB"
		}
		if product.Key == "" {
			return nil, nil, errors.New("payment product key is empty")
		}
		if product.Title == "" {
			return nil, nil, fmt.Errorf("payment product %q has empty title", product.Key)
		}
		if product.Coins <= 0 {
			return nil, nil, fmt.Errorf("payment product %q has invalid coins", product.Key)
		}
		if product.Amount <= 0 {
			return nil, nil, fmt.Errorf("payment product %q has invalid amount", product.Key)
		}
		if _, exists := outByKey[product.Key]; exists {
			return nil, nil, fmt.Errorf("duplicate payment product key %q", product.Key)
		}
		out = append(out, product)
		outByKey[product.Key] = product
	}
	return out, outByKey, nil
}

func normalizeKey(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func normalizeOptionalURL(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", nil
	}
	u, err := url.ParseRequestURI(value)
	if err != nil {
		return "", err
	}
	scheme := strings.ToLower(strings.TrimSpace(u.Scheme))
	if scheme != "http" && scheme != "https" {
		return "", errors.New("url must start with http:// or https://")
	}
	if strings.TrimSpace(u.Host) == "" {
		return "", errors.New("url host is empty")
	}
	return u.String(), nil
}
