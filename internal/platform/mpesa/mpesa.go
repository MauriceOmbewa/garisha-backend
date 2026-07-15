// Package mpesa provides a lightweight Safaricom Daraja API client.
// It supports the Lipa Na M-PESA Online (STK Push) flow used for
// collecting payments directly from a customer's phone.
//
// Flow:
//  1. Call STKPush → Safaricom sends a payment prompt to the customer's phone.
//  2. Customer enters PIN → Safaricom POSTs the result to CallbackURL.
//  3. Your handler calls ParseCallback to decode the result and update the
//     payment record status.
//
// Only sandbox and production environments are supported. The base URL is
// chosen automatically from the Environment field in MpesaConfig.
package mpesa

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ── Client ────────────────────────────────────────────────────────────────────

// Client is the Daraja API client.
type Client struct {
	consumerKey    string
	consumerSecret string
	passkey        string
	shortCode      string
	callbackURL    string
	baseURL        string
	httpClient     *http.Client
}

const (
	sandboxBaseURL    = "https://sandbox.safaricom.co.ke"
	productionBaseURL = "https://api.safaricom.co.ke"
)

// New creates a Daraja client.  environment must be "sandbox" or "production".
// If consumerKey is empty the client is created in a no-op mode so the app
// starts even when M-PESA is not configured.
func New(consumerKey, consumerSecret, passkey, shortCode, callbackURL, environment string) *Client {
	base := sandboxBaseURL
	if environment == "production" {
		base = productionBaseURL
	}

	return &Client{
		consumerKey:    consumerKey,
		consumerSecret: consumerSecret,
		passkey:        passkey,
		shortCode:      shortCode,
		callbackURL:    callbackURL,
		baseURL:        base,
		httpClient:     &http.Client{Timeout: 30 * time.Second},
	}
}

// Enabled reports whether the client has credentials configured.
func (c *Client) Enabled() bool {
	return c.consumerKey != "" && c.consumerSecret != ""
}

// ── OAuth token ───────────────────────────────────────────────────────────────

// accessToken fetches a short-lived OAuth2 bearer token from Daraja.
func (c *Client) accessToken(ctx context.Context) (string, error) {
	url := c.baseURL + "/oauth/v1/generate?grant_type=client_credentials"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("mpesa: build token request: %w", err)
	}

	creds := base64.StdEncoding.EncodeToString(
		[]byte(c.consumerKey + ":" + c.consumerSecret),
	)
	req.Header.Set("Authorization", "Basic "+creds)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("mpesa: token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("mpesa: token HTTP %d: %s", resp.StatusCode, body)
	}

	var result struct {
		AccessToken string `json:"access_token"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("mpesa: decode token response: %w", err)
	}

	return result.AccessToken, nil
}

// ── STK Push ──────────────────────────────────────────────────────────────────

// STKPushRequest contains the parameters for a Lipa Na M-PESA Online request.
type STKPushRequest struct {
	// Phone number in international format without the '+', e.g. "254712345678"
	PhoneNumber string
	// Amount in KES (whole units, no decimals)
	Amount int64
	// AccountReference appears on the customer's M-PESA statement
	AccountReference string
	// TransactionDesc is a short description (≤13 chars recommended)
	TransactionDesc string
}

// STKPushResponse is the immediate Daraja acknowledgement.
// A 200 OK with ResponseCode "0" means the prompt was sent — NOT that
// the payment succeeded.  The final result arrives via callback.
type STKPushResponse struct {
	MerchantRequestID   string `json:"MerchantRequestID"`
	CheckoutRequestID   string `json:"CheckoutRequestID"`
	ResponseCode        string `json:"ResponseCode"`
	ResponseDescription string `json:"ResponseDescription"`
	CustomerMessage     string `json:"CustomerMessage"`
}

// STKPush initiates a Lipa Na M-PESA Online (STK Push) payment.
// Returns the CheckoutRequestID which should be stored against the payment
// record so it can be matched to the callback.
func (c *Client) STKPush(ctx context.Context, req STKPushRequest) (*STKPushResponse, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("mpesa: client not configured")
	}

	token, err := c.accessToken(ctx)
	if err != nil {
		return nil, err
	}

	timestamp := time.Now().Format("20060102150405")
	password := base64.StdEncoding.EncodeToString(
		[]byte(c.shortCode + c.passkey + timestamp),
	)

	body := map[string]any{
		"BusinessShortCode": c.shortCode,
		"Password":          password,
		"Timestamp":         timestamp,
		"TransactionType":   "CustomerPayBillOnline",
		"Amount":            req.Amount,
		"PartyA":            req.PhoneNumber,
		"PartyB":            c.shortCode,
		"PhoneNumber":       req.PhoneNumber,
		"CallBackURL":       c.callbackURL,
		"AccountReference":  req.AccountReference,
		"TransactionDesc":   req.TransactionDesc,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("mpesa: marshal stk request: %w", err)
	}

	url := c.baseURL + "/mpesa/stkpush/v1/processrequest"

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("mpesa: build stk request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("mpesa: stk push request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("mpesa: read stk response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mpesa: stk push HTTP %d: %s", resp.StatusCode, respBody)
	}

	var result STKPushResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("mpesa: decode stk response: %w", err)
	}

	if result.ResponseCode != "0" {
		return nil, fmt.Errorf("mpesa: stk push failed: %s", result.ResponseDescription)
	}

	return &result, nil
}

// ── Callback parsing ──────────────────────────────────────────────────────────

// CallbackResult is the decoded body of a Daraja STK Push callback.
type CallbackResult struct {
	CheckoutRequestID string
	ResultCode        int
	ResultDesc        string
	// Populated only when ResultCode == 0 (success)
	MpesaReceiptNumber string
	Amount             float64
	PhoneNumber        string
	TransactionDate    time.Time
}

// ParseCallback decodes the raw JSON body sent by Safaricom to the callback URL.
func ParseCallback(body []byte) (*CallbackResult, error) {
	// Safaricom wraps the result inside Body.stkCallback.
	var raw struct {
		Body struct {
			StkCallback struct {
				MerchantRequestID string `json:"MerchantRequestID"`
				CheckoutRequestID string `json:"CheckoutRequestID"`
				ResultCode        int    `json:"ResultCode"`
				ResultDesc        string `json:"ResultDesc"`
				CallbackMetadata  *struct {
					Item []struct {
						Name  string `json:"Name"`
						Value any    `json:"Value"`
					} `json:"Item"`
				} `json:"CallbackMetadata"`
			} `json:"stkCallback"`
		} `json:"Body"`
	}

	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("mpesa: parse callback: %w", err)
	}

	cb := raw.Body.StkCallback

	result := &CallbackResult{
		CheckoutRequestID: cb.CheckoutRequestID,
		ResultCode:        cb.ResultCode,
		ResultDesc:        cb.ResultDesc,
	}

	// Extract metadata items only present on success (ResultCode 0).
	if cb.CallbackMetadata != nil {
		for _, item := range cb.CallbackMetadata.Item {
			switch item.Name {
			case "MpesaReceiptNumber":
				if v, ok := item.Value.(string); ok {
					result.MpesaReceiptNumber = v
				}
			case "Amount":
				switch v := item.Value.(type) {
				case float64:
					result.Amount = v
				case json.Number:
					f, _ := v.Float64()
					result.Amount = f
				}
			case "PhoneNumber":
				switch v := item.Value.(type) {
				case string:
					result.PhoneNumber = v
				case float64:
					result.PhoneNumber = fmt.Sprintf("%.0f", v)
				}
			case "TransactionDate":
				switch v := item.Value.(type) {
				case float64:
					t, err := time.Parse("20060102150405", fmt.Sprintf("%.0f", v))
					if err == nil {
						result.TransactionDate = t
					}
				case string:
					t, err := time.Parse("20060102150405", v)
					if err == nil {
						result.TransactionDate = t
					}
				}
			}
		}
	}

	return result, nil
}
