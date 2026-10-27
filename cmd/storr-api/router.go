package main

import (
	"storr-api/internal/auth"
	"storr-api/internal/integration"
	"storr-api/internal/pipeline"
	"storr-api/internal/product"
	"storr-api/internal/product/image"
	"storr-api/internal/product/webhook"
	"storr-api/internal/store"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func newRouter(deps allDeps, corsOrigin string) *chi.Mux {
	r := chi.NewRouter()
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{corsOrigin},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization", "X-Requested-With"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	registerOAuthCallbackRoutes(r, deps.integration.callbackHandler)
	registerWebhookRoutes(r, deps.product.webhookHandler)
	registerAuthRoutes(r, deps.auth)
	registerStoreRoutes(r, deps)
	return r
}

func registerOAuthCallbackRoutes(r *chi.Mux, h *integration.CallbackHandler) {
	r.Route("/oauth", func(r chi.Router) {
		r.Get("/figma/callback", h.FigmaCallback)
		r.Get("/shopify/callback", h.ShopifyCallback)
	})
}

func registerWebhookRoutes(r *chi.Mux, h *webhook.Handler) {
	r.Post("/webhooks/shopify/products", h.HandleProductWebhook)
}

func registerAuthRoutes(r *chi.Mux, a authDeps) {
	r.Route("/auth", func(r chi.Router) {
		r.Post("/signup", a.handler.SignUp)
		r.Post("/login", a.handler.Login)
		r.Post("/logout", a.handler.Logout)
		r.Post("/refresh", a.handler.Refresh)
		r.Get("/me", a.handler.Me)
	})
}

func registerStoreRoutes(r *chi.Mux, deps allDeps) {
	r.Route("/stores", func(r chi.Router) {
		r.Use(auth.RequireAuth(deps.auth.service))
		r.Get("/", deps.store.handler.List)
		r.Post("/", deps.store.handler.Create)
		r.Route("/{id}", func(r chi.Router) {
			r.Use(store.RequireStoreOwnership(deps.store.service))
			registerStoreCRUD(r, deps.store.handler)
			registerPipelineEndpoints(r, deps.pipeline.handler)
			registerStoreIntegrations(r, deps.integration.handler)
			registerProductRoutes(r, deps.product.handler, deps.product.imageHandler)
		})
	})
}

func registerStoreCRUD(r chi.Router, h *store.Handler) {
	r.Get("/", h.Get)
	r.Put("/", h.Update)
	r.Delete("/", h.Delete)
}

func registerPipelineEndpoints(r chi.Router, h *pipeline.Handler) {
	r.Post("/provision", h.Provision)
	r.Get("/status", h.GetStatus)
	r.Get("/readiness", h.GetReadiness)
	r.Post("/select-theme", h.SelectTheme)
	r.Post("/provide-input", h.ProvideInput)
}

func registerProductRoutes(r chi.Router, h *product.Handler, ih *image.Handler) {
	r.Route("/products", func(r chi.Router) {
		r.Get("/template", h.DownloadTemplate)
		r.Post("/import", h.Import)
		r.Post("/sync/pull", h.SyncPull)
		r.Post("/sync/push", h.SyncPush)
		r.Put("/batch", h.BatchUpdate)
		r.Get("/", h.List)
		r.Post("/", h.Create)
		r.Get("/{productId}", h.Get)
		r.Put("/{productId}", h.Update)
		r.Delete("/{productId}", h.Delete)
		r.Post("/{productId}/sync/push", h.SyncPushOne)
		r.Get("/{productId}/shopify-version", h.GetShopifyVersion)
		r.Post("/{productId}/resolve-conflict", h.ResolveConflict)
		registerImageRoutes(r, ih)
	})
}

func registerImageRoutes(r chi.Router, ih *image.Handler) {
	r.Post("/{productId}/images/upload-url", ih.GenerateUploadURL)
	r.Post("/{productId}/images", ih.ConfirmUpload)
	r.Put("/{productId}/images/reorder", ih.ReorderImages)
	r.Put("/{productId}/images/{imageId}", ih.UpdateImage)
	r.Delete("/{productId}/images/{imageId}", ih.DeleteImage)
}

func registerStoreIntegrations(r chi.Router, h *integration.Handler) {
	r.Get("/integrations", h.ListStoreIntegrations)
	r.Put("/integrations/{key}", h.UpdateStoreIntegration)
	r.Delete("/integrations/{key}", h.DisableStoreIntegration)
	r.Post("/integrations/figma/start", h.FigmaStart)
	r.Post("/integrations/shopify/start", h.ShopifyStart)
}
