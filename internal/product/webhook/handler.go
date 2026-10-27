package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"log/slog"
	"net/http"

	"storr-api/internal/api"
	"storr-api/internal/product"
)

// StoreFinder resolves a Shopify domain URL to a store's owner and store IDs.
type StoreFinder interface {
	FindByShopifyURL(ctx context.Context, shopifyURL string) (string, string, error)
}

// Handler handles incoming Shopify product webhooks.
type Handler struct {
	service      *Service
	eventStore   *EventStore
	clientSecret string
}

// NewHandler creates a webhook handler with the required dependencies.
func NewHandler(service *Service, eventStore *EventStore, clientSecret string) *Handler {
	return &Handler{service: service, eventStore: eventStore, clientSecret: clientSecret}
}

// HandleProductWebhook validates and processes a Shopify product webhook.
func (h *Handler) HandleProductWebhook(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, product.WebhookMaxBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "Failed to read body", "INVALID_BODY")
		return
	}
	if !h.verifyHMAC(body, r.Header.Get("X-Shopify-Hmac-SHA256")) {
		api.WriteError(w, http.StatusUnauthorized, "Invalid signature", "INVALID_SIGNATURE")
		return
	}
	h.processWebhook(r.Context(), w, r, body)
}

func (h *Handler) processWebhook(ctx context.Context, w http.ResponseWriter, r *http.Request, body []byte) {
	eventID := r.Header.Get("X-Shopify-Event-Id")
	if eventID != "" {
		isNew, err := h.eventStore.TryRecord(ctx, eventID)
		if err != nil {
			slog.Warn("webhook dedup check failed", "eventId", eventID, "error", err)
		}
		if !isNew && err == nil {
			api.WriteSuccessOK(w)
			return
		}
	}
	domain := "https://" + r.Header.Get("X-Shopify-Shop-Domain")
	topic := r.Header.Get("X-Shopify-Topic")
	if err := h.routeByTopic(ctx, domain, topic, body); err != nil {
		slog.Warn("webhook failed", "topic", topic, "error", err)
	}
	api.WriteSuccessOK(w)
}

func (h *Handler) routeByTopic(ctx context.Context, domain, topic string, body []byte) error {
	switch topic {
	case "products/create", "products/update":
		return h.service.HandleUpsert(ctx, domain, body)
	case "products/delete":
		return h.service.HandleDelete(ctx, domain, body)
	default:
		return nil
	}
}

// --- HMAC Validation ---

func (h *Handler) verifyHMAC(body []byte, headerMAC string) bool {
	if headerMAC == "" || h.clientSecret == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(h.clientSecret))
	mac.Write(body)
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(headerMAC))
}
