package ownernotification

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTelnyxSenderAcceptedAndDelivered(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("missing authorization")
		}
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"data":{"id":"message-1","to":[{"status":"queued"}]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"to":[{"status":"delivered"}]}}`))
	}))
	defer server.Close()
	sender := TelnyxSender{APIKey: "test-key", FromNumber: "+15550000000", Client: server.Client(), BaseURL: server.URL}
	sent, err := sender.Send(context.Background(), "+15550000001", "test")
	if err != nil || sent.Disposition != "accepted" || sent.ProviderMessageID != "message-1" {
		t.Fatalf("send = %#v, %v", sent, err)
	}
	delivery, err := sender.Retrieve(context.Background(), sent.ProviderMessageID)
	if err != nil || delivery.Disposition != "delivered" {
		t.Fatalf("delivery = %#v, %v", delivery, err)
	}
}

func TestTelnyxSenderDoesNotRetryAmbiguousServerFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()
	sender := TelnyxSender{APIKey: "test-key", FromNumber: "+15550000000", Client: server.Client(), BaseURL: server.URL}
	result, err := sender.Send(context.Background(), "+15550000001", "test")
	if err != nil || result.Disposition != "ambiguous" {
		t.Fatalf("send = %#v, %v", result, err)
	}
}

func TestTelnyxSenderTreatsFinalUnconfirmedAsUnknownDelivery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"to":[{"status":"delivery_unconfirmed"}]}}`))
	}))
	defer server.Close()
	sender := TelnyxSender{APIKey: "test-key", FromNumber: "+15550000000", Client: server.Client(), BaseURL: server.URL}
	result, err := sender.Retrieve(context.Background(), "message-1")
	if err != nil || result.Disposition != "unconfirmed" {
		t.Fatalf("delivery = %#v, %v", result, err)
	}
}
