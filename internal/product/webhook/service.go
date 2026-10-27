package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"storr-api/internal/product"

	"github.com/google/uuid"
)

// Service contains the business logic for processing Shopify product webhooks.
type Service struct {
	repo        *product.Repository
	storeFinder StoreFinder
}

// NewService creates a webhook service with the required dependencies.
func NewService(repo *product.Repository, storeFinder StoreFinder) *Service {
	return &Service{repo: repo, storeFinder: storeFinder}
}

// HandleUpsert processes a products/create or products/update webhook payload.
func (s *Service) HandleUpsert(ctx context.Context, domain string, body []byte) error {
	var wp webhookProduct
	if err := json.Unmarshal(body, &wp); err != nil {
		return fmt.Errorf("unmarshal product: %w", err)
	}
	_, storeID, err := s.storeFinder.FindByShopifyURL(ctx, domain)
	if err != nil {
		return fmt.Errorf("find store: %w", err)
	}
	pulled := webhookProductToStorr(wp)
	return s.upsertWebhookProduct(ctx, storeID, pulled)
}

// HandleDelete processes a products/delete webhook payload.
func (s *Service) HandleDelete(ctx context.Context, domain string, body []byte) error {
	var payload webhookDeletePayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("unmarshal delete payload: %w", err)
	}
	_, storeID, err := s.storeFinder.FindByShopifyURL(ctx, domain)
	if err != nil {
		return fmt.Errorf("find store: %w", err)
	}
	return s.deleteByShopifyID(ctx, storeID, shopifyProductGID(payload.ID))
}

func (s *Service) upsertWebhookProduct(ctx context.Context, storeID string, pulled product.Product) error {
	existing, err := s.repo.GetByShopifyID(ctx, storeID, pulled.ShopifyID)
	if err != nil && !errors.Is(err, product.ErrProductNotFound) {
		return err
	}
	if existing != nil {
		if isEchoWebhook(existing) {
			return nil
		}
		return s.updateFromWebhook(ctx, *existing, pulled)
	}
	return s.createFromWebhook(ctx, storeID, pulled)
}

func isEchoWebhook(p *product.Product) bool {
	if p.LastPushedAt == nil {
		return false
	}
	return time.Since(*p.LastPushedAt).Seconds() < product.WebhookEchoWindowSeconds
}

func (s *Service) updateFromWebhook(ctx context.Context, existing product.Product, pulled product.Product) error {
	now := time.Now().UTC()
	if existing.SyncStatus == product.SyncPending {
		return s.markConflict(ctx, existing, now)
	}
	updated := product.ApplyPulledFields(existing, pulled)
	updated.LastPulledAt = &now
	updated.SyncStatus = product.SyncSynced
	updated.UpdatedAt = now
	return s.repo.Update(ctx, updated)
}

func (s *Service) markConflict(ctx context.Context, existing product.Product, now time.Time) error {
	existing.SyncStatus = product.SyncConflict
	existing.UpdatedAt = now
	return s.repo.Update(ctx, existing)
}

func (s *Service) createFromWebhook(ctx context.Context, storeID string, pulled product.Product) error {
	now := time.Now().UTC()
	pulled.StoreID = storeID
	pulled.ProductID = product.ProductIDPrefix + uuid.NewString()
	pulled.LastPulledAt = &now
	pulled.SyncStatus = product.SyncSynced
	pulled.CreatedAt = now
	pulled.UpdatedAt = now
	return s.repo.Create(ctx, pulled)
}

func (s *Service) deleteByShopifyID(ctx context.Context, storeID, shopifyID string) error {
	existing, err := s.repo.GetByShopifyID(ctx, storeID, shopifyID)
	if err != nil {
		if errors.Is(err, product.ErrProductNotFound) {
			return nil
		}
		return err
	}
	return s.repo.Delete(ctx, storeID, existing.ProductID)
}
