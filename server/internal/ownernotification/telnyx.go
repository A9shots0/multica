package ownernotification

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const telnyxAPIBase = "https://api.telnyx.com/v2"

type SendResult struct {
	ProviderMessageID string
	ProviderState     string
	Disposition       string // accepted, retryable, rejected, ambiguous
	ErrorCode         string
}

type DeliveryResult struct {
	ProviderState string
	Disposition   string // pending, delivered, failed, unconfirmed, ambiguous
}

type Sender interface {
	Send(context.Context, string, string) (SendResult, error)
	Retrieve(context.Context, string) (DeliveryResult, error)
}

type TelnyxSender struct {
	APIKey             string
	FromNumber         string
	MessagingProfileID string
	Client             *http.Client
	BaseURL            string
}

func (sender TelnyxSender) client() *http.Client {
	if sender.Client != nil {
		return sender.Client
	}
	return &http.Client{Timeout: 15 * time.Second}
}

func (sender TelnyxSender) baseURL() string {
	if strings.TrimSpace(sender.BaseURL) != "" {
		return strings.TrimRight(sender.BaseURL, "/")
	}
	return telnyxAPIBase
}

func (sender TelnyxSender) Send(ctx context.Context, to, message string) (SendResult, error) {
	if strings.TrimSpace(sender.APIKey) == "" || strings.TrimSpace(sender.FromNumber) == "" || strings.TrimSpace(to) == "" {
		return SendResult{Disposition: "rejected", ErrorCode: "provider_not_configured"}, nil
	}
	body := map[string]string{"to": to, "from": sender.FromNumber, "text": message}
	if strings.TrimSpace(sender.MessagingProfileID) != "" {
		body["messaging_profile_id"] = sender.MessagingProfileID
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return SendResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sender.baseURL()+"/messages", bytes.NewReader(encoded))
	if err != nil {
		return SendResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+sender.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := sender.client().Do(req)
	if err != nil {
		return SendResult{Disposition: "ambiguous", ErrorCode: "transport_error"}, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return SendResult{Disposition: "ambiguous", ErrorCode: "response_read_error"}, err
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return SendResult{Disposition: "retryable", ErrorCode: "rate_limited"}, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		disposition := "rejected"
		if resp.StatusCode >= 500 {
			disposition = "ambiguous"
		}
		return SendResult{Disposition: disposition, ErrorCode: fmt.Sprintf("http_%d", resp.StatusCode)}, nil
	}
	var envelope struct {
		Data struct {
			ID string `json:"id"`
			To []struct {
				Status string `json:"status"`
			} `json:"to"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || strings.TrimSpace(envelope.Data.ID) == "" {
		return SendResult{Disposition: "ambiguous", ErrorCode: "invalid_provider_response"}, errors.New("Telnyx accepted the request but returned no usable message identity")
	}
	state := "accepted"
	if len(envelope.Data.To) > 0 && strings.TrimSpace(envelope.Data.To[0].Status) != "" {
		state = envelope.Data.To[0].Status
	}
	return SendResult{ProviderMessageID: envelope.Data.ID, ProviderState: state, Disposition: "accepted"}, nil
}

func (sender TelnyxSender) Retrieve(ctx context.Context, providerMessageID string) (DeliveryResult, error) {
	if strings.TrimSpace(sender.APIKey) == "" || strings.TrimSpace(providerMessageID) == "" {
		return DeliveryResult{Disposition: "ambiguous", ProviderState: "missing_provider_identity"}, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sender.baseURL()+"/messages/"+providerMessageID, nil)
	if err != nil {
		return DeliveryResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+sender.APIKey)
	resp, err := sender.client().Do(req)
	if err != nil {
		return DeliveryResult{Disposition: "ambiguous", ProviderState: "transport_error"}, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return DeliveryResult{Disposition: "ambiguous", ProviderState: "response_read_error"}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return DeliveryResult{Disposition: "ambiguous", ProviderState: fmt.Sprintf("http_%d", resp.StatusCode)}, nil
	}
	var envelope struct {
		Data struct {
			To []struct {
				Status string `json:"status"`
			} `json:"to"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || len(envelope.Data.To) == 0 {
		return DeliveryResult{Disposition: "ambiguous", ProviderState: "invalid_provider_response"}, err
	}
	state := strings.ToLower(strings.TrimSpace(envelope.Data.To[0].Status))
	switch state {
	case "delivered":
		return DeliveryResult{Disposition: "delivered", ProviderState: state}, nil
	case "delivery_failed", "sending_failed", "failed", "gw_timeout":
		return DeliveryResult{Disposition: "failed", ProviderState: state}, nil
	case "delivery_unconfirmed", "dlr_timeout":
		return DeliveryResult{Disposition: "unconfirmed", ProviderState: state}, nil
	case "queued", "sending", "sent", "accepted":
		return DeliveryResult{Disposition: "pending", ProviderState: state}, nil
	default:
		return DeliveryResult{Disposition: "ambiguous", ProviderState: state}, nil
	}
}
