package platega

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/elum-bots/core/payments"
)

type Client struct {
	baseURL    string
	merchantID string
	secret     string
	httpClient *http.Client
}

func NewClient(baseURL, merchantID, secret string) (*Client, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("platega base url is empty")
	}
	if strings.TrimSpace(merchantID) == "" || strings.TrimSpace(secret) == "" {
		return nil, fmt.Errorf("platega credentials are empty")
	}
	return &Client{
		baseURL:    baseURL,
		merchantID: strings.TrimSpace(merchantID),
		secret:     strings.TrimSpace(secret),
		httpClient: &http.Client{Timeout: 20 * time.Second},
	}, nil
}

type createReq struct {
	PaymentMethod  int `json:"paymentMethod"`
	PaymentDetails struct {
		Amount   int64  `json:"amount"`
		Currency string `json:"currency"`
	} `json:"paymentDetails"`
	Description string `json:"description,omitempty"`
	Return      string `json:"return,omitempty"`
	FailedURL   string `json:"failedUrl,omitempty"`
	Payload     string `json:"payload,omitempty"`
}

type createResp struct {
	TransactionID string `json:"transactionId"`
	Redirect      string `json:"redirect"`
	Status        string `json:"status"`
}

func (c *Client) CreateTransaction(ctx context.Context, in payments.CreateRequest) (payments.CreateResult, error) {
	reqBody := createReq{
		PaymentMethod: in.PaymentMethod,
		Description:   strings.TrimSpace(in.Description),
		Return:        strings.TrimSpace(in.ReturnURL),
		FailedURL:     strings.TrimSpace(in.FailedURL),
		Payload:       strings.TrimSpace(in.Payload),
	}
	reqBody.PaymentDetails.Amount = in.Amount
	reqBody.PaymentDetails.Currency = strings.TrimSpace(in.Currency)
	if reqBody.PaymentDetails.Currency == "" {
		reqBody.PaymentDetails.Currency = "RUB"
	}

	raw, err := json.Marshal(reqBody)
	if err != nil {
		return payments.CreateResult{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/transaction/process", bytes.NewReader(raw))
	if err != nil {
		return payments.CreateResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-MerchantId", c.merchantID)
	req.Header.Set("X-Secret", c.secret)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return payments.CreateResult{}, err
	}
	defer resp.Body.Close()

	respRaw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return payments.CreateResult{}, fmt.Errorf("platega create transaction failed: http=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respRaw)))
	}

	var out createResp
	if err := json.Unmarshal(respRaw, &out); err != nil {
		return payments.CreateResult{}, err
	}

	txID := strings.TrimSpace(out.TransactionID)
	redirectURL := strings.TrimSpace(out.Redirect)
	if txID == "" || redirectURL == "" {
		return payments.CreateResult{}, fmt.Errorf("platega create transaction returned empty fields: %s", strings.TrimSpace(string(respRaw)))
	}

	return payments.CreateResult{
		TransactionID: txID,
		RedirectURL:   redirectURL,
		Status:        strings.TrimSpace(out.Status),
	}, nil
}
