package app

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	elumbot "github.com/elum-bots/core/internal/bot"
	maxadapter "github.com/elum-bots/core/internal/bot/adapters/max"
	"github.com/elum-bots/core/internal/db"
	platega "github.com/elum-bots/core/internal/integration/platega"
	"github.com/elum-bots/core/payments"
)

type plategaCallbackPayload struct {
	ID            string  `json:"id"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	Status        string  `json:"status"`
	PaymentMethod int     `json:"paymentMethod"`
	MerchantID    string  `json:"merchantId"`
	Secret        string  `json:"secret"`
}

func newPaymentService(cfg Config, store *db.Store, opts options) (*payments.Service, error) {
	service, err := payments.NewService(store, payments.Config{
		Enabled:       cfg.FeaturePayments,
		PaymentMethod: cfg.PlategaPaymentMethod,
		ReturnURL:     cfg.PlategaReturnURL,
		FailedURL:     cfg.PlategaFailedURL,
		Products:      opts.paymentProducts,
	})
	if err != nil {
		return nil, err
	}

	if cfg.FeaturePayments {
		gateway, err := platega.NewClient(cfg.PlategaBaseURL, cfg.PlategaMerchantID, cfg.PlategaSecret)
		if err != nil {
			log.Printf("payments gateway init failed: %v", err)
		} else {
			service.SetGateway(gateway)
		}
	}

	return service, nil
}

func newHTTPServer(cfg Config, paymentSvc *payments.Service, adapter *maxadapter.Adapter, runtimeBot *elumbot.Bot, deps Dependencies, opts options) *http.Server {
	return &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           runtimeHTTPHandler(cfg, paymentSvc, adapter, runtimeBot, deps, opts),
		ReadHeaderTimeout: 5 * time.Second,
	}
}

func runtimeHTTPHandler(cfg Config, paymentSvc *payments.Service, adapter *maxadapter.Adapter, runtimeBot *elumbot.Bot, deps Dependencies, opts options) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	if paymentSvc != nil && paymentSvc.IsEnabled() {
		mux.HandleFunc("/payments/platega/callback", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}

			defer r.Body.Close()
			var in plategaCallbackPayload
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
				http.Error(w, "invalid callback payload", http.StatusBadRequest)
				return
			}
			if !validPlategaPayload(in) {
				http.Error(w, "invalid callback payload", http.StatusBadRequest)
				return
			}
			if !isAuthorizedPlategaCallback(cfg, in, r) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			tx, rewardedNow, err := paymentSvc.HandleCallback(r.Context(), payments.Callback{
				ID:            in.ID,
				Amount:        in.Amount,
				Currency:      in.Currency,
				Status:        in.Status,
				PaymentMethod: in.PaymentMethod,
			})
			if err != nil {
				http.Error(w, "invalid callback payload", http.StatusBadRequest)
				return
			}
			if rewardedNow {
				log.Printf("payment confirmed user=%s product=%s tx=%s coins=%d", tx.UserID, tx.ProductKey, tx.TransactionID, tx.Coins)
				for _, hook := range opts.paymentHooks {
					if hook == nil {
						continue
					}
					if err := hook(r.Context(), runtimeBot, deps, tx); err != nil {
						log.Printf("payment success hook error: user=%s tx=%s err=%v", tx.UserID, tx.TransactionID, err)
					}
				}
			}

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		})
	}

	if adapter != nil {
		mux.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}

			if cfg.WebhookSecret != "" {
				reqSecret := strings.TrimSpace(r.Header.Get("X-Max-Bot-Api-Secret"))
				if reqSecret != cfg.WebhookSecret {
					http.Error(w, "unauthorized", http.StatusUnauthorized)
					return
				}
			}

			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}

			if err := adapter.HandleWebhook(body); err != nil {
				log.Printf("max webhook error: %v", err)
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		})
	}

	return mux
}

func isAuthorizedPlategaCallback(cfg Config, in plategaCallbackPayload, r *http.Request) bool {
	expectedMerchant := strings.TrimSpace(cfg.PlategaMerchantID)
	expectedSecret := strings.TrimSpace(cfg.PlategaSecret)
	if expectedMerchant == "" || expectedSecret == "" {
		return true
	}

	merchant := strings.TrimSpace(r.Header.Get("X-MerchantId"))
	secret := strings.TrimSpace(r.Header.Get("X-Secret"))
	if merchant == "" {
		merchant = strings.TrimSpace(in.MerchantID)
	}
	if secret == "" {
		secret = strings.TrimSpace(in.Secret)
	}
	return merchant == expectedMerchant && secret == expectedSecret
}

func validPlategaPayload(v plategaCallbackPayload) bool {
	if strings.TrimSpace(v.ID) == "" {
		return false
	}
	if strings.TrimSpace(v.Currency) == "" {
		return false
	}
	status := strings.ToUpper(strings.TrimSpace(v.Status))
	if status != "CONFIRMED" && status != "CANCELED" {
		return false
	}
	switch v.PaymentMethod {
	case 2, 11, 12, 13:
		return true
	default:
		return false
	}
}
