package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"storr-api/internal/api"
	"storr-api/internal/auth"
	"storr-api/internal/infra"
	"storr-api/internal/store"

	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/go-chi/chi/v5"
)

// EnvOverrideBuilder assembles ECS environment variable overrides from integration secrets.
type EnvOverrideBuilder interface {
	BuildEnvOverrides(ctx context.Context, pc store.ProvisionContext) ([]types.KeyValuePair, error)
}

// Services groups the pipeline services needed by the Handler.
type Services struct {
	Pipeline *Service
	Theme    *ThemeService
	Stores   *store.Service
}

// CleanupDeps holds optional infrastructure managers for store cleanup.
type CleanupDeps struct {
	PreviewRoute *infra.PreviewRouteManager
	Efs          *infra.EfsProvisioner
}

// Handler provides HTTP endpoints for pipeline provisioning and status.
type Handler struct {
	services    Services
	provisioner EnvOverrideBuilder
	cleanup     CleanupDeps
}

// NewHandler creates a pipeline Handler with the given services and infrastructure deps.
func NewHandler(services Services, provisioner EnvOverrideBuilder, cleanup CleanupDeps) *Handler {
	return &Handler{
		services:    services,
		provisioner: provisioner,
		cleanup:     cleanup,
	}
}

// CleanupStore stops running tasks and removes infrastructure for a deleted store.
func (h *Handler) CleanupStore(ctx context.Context, storeID string) {
	h.services.Pipeline.StopTask(ctx, storeID)
	h.services.Theme.StopTask(ctx, storeID)
	if h.cleanup.PreviewRoute != nil {
		h.cleanup.PreviewRoute.DeletePreviewRoute(ctx, storeID)
	}
	if h.cleanup.Efs != nil {
		h.cleanup.Efs.DeleteAccessPoint(ctx, storeID)
	}
}

// Provision starts or returns the status of a build pipeline for a store.
func (h *Handler) Provision(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromRequest(r)
	storeID := chi.URLParam(r, "id")
	s, err := h.services.Stores.Get(r.Context(), user.ID, storeID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	h.executeProvision(w, r, s.PipelineContext(), s.ProvisionContext())
}

func (h *Handler) executeProvision(w http.ResponseWriter, r *http.Request, pc store.PipelineContext, prov store.ProvisionContext) {
	readiness := store.CheckReadiness(prov.Integrations)
	if !readiness.Ready {
		writeReadinessError(w, readiness)
		return
	}
	switch pc.ProjectType {
	case store.ProjectTypeTheme:
		h.runThemePipeline(w, r, pc, prov)
	default:
		h.runPipeline(w, r, pc, prov)
	}
}

func (h *Handler) runPipeline(w http.ResponseWriter, r *http.Request, pc store.PipelineContext, prov store.ProvisionContext) {
	envOverrides, err := h.provisioner.BuildEnvOverrides(r.Context(), prov)
	if err != nil {
		api.WriteInternalError(w, "Failed to build env overrides", err)
		return
	}
	resp, err := h.services.Pipeline.Provision(r.Context(), pc.StoreID, envOverrides)
	if err != nil {
		api.WriteInternalError(w, "Failed to provision pipeline", err)
		return
	}
	data, err := json.Marshal(resp)
	if err != nil {
		api.WriteMarshalError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, data)
}

func (h *Handler) runThemePipeline(w http.ResponseWriter, r *http.Request, pc store.PipelineContext, prov store.ProvisionContext) {
	envOverrides, err := h.provisioner.BuildEnvOverrides(r.Context(), prov)
	if err != nil {
		api.WriteInternalError(w, "Failed to build env overrides", err)
		return
	}
	resp, err := h.services.Theme.Provision(r.Context(), pc.StoreID, pc.ShopifyURL, envOverrides)
	if err != nil {
		api.WriteInternalError(w, "Failed to provision theme", err)
		return
	}
	data, err := json.Marshal(resp)
	if err != nil {
		api.WriteMarshalError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, data)
}

func writeReadinessError(w http.ResponseWriter, r *store.ReadinessResponse) {
	msg := readinessMessage(r)
	api.WriteError(w, http.StatusBadRequest, msg, "INTEGRATION_REQUIRED")
}

func readinessMessage(r *store.ReadinessResponse) string {
	if !r.Figma.Connected {
		return "Figma integration not connected"
	}
	return "Shopify integration not connected"
}

// GetStatus returns the current pipeline status for a store.
func (h *Handler) GetStatus(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromRequest(r)
	storeID := chi.URLParam(r, "id")
	s, err := h.services.Stores.Get(r.Context(), user.ID, storeID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	pc := s.PipelineContext()
	h.writeStatus(w, r, pc)
}

func (h *Handler) writeStatus(w http.ResponseWriter, r *http.Request, pc store.PipelineContext) {
	switch pc.ProjectType {
	case store.ProjectTypeTheme:
		h.writeThemeStatus(w, r, pc.StoreID)
	default:
		h.writeHydrogenStatus(w, r, pc.StoreID)
	}
}

func (h *Handler) writeHydrogenStatus(w http.ResponseWriter, r *http.Request, storeID string) {
	resp, err := h.services.Pipeline.GetStatus(r.Context(), storeID)
	if err != nil {
		api.WriteInternalError(w, "Failed to get status", err)
		return
	}
	data, err := json.Marshal(resp)
	if err != nil {
		api.WriteMarshalError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, data)
}

func (h *Handler) writeThemeStatus(w http.ResponseWriter, r *http.Request, storeID string) {
	resp, err := h.services.Theme.GetStatus(r.Context(), storeID)
	if err != nil {
		api.WriteInternalError(w, "Failed to get status", err)
		return
	}
	data, err := json.Marshal(resp)
	if err != nil {
		api.WriteMarshalError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, data)
}

// GetReadiness checks whether all required integrations are connected for a store.
func (h *Handler) GetReadiness(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromRequest(r)
	storeID := chi.URLParam(r, "id")
	s, err := h.services.Stores.Get(r.Context(), user.ID, storeID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	readiness := store.CheckReadiness(s.Integrations)
	data, err := json.Marshal(readiness)
	if err != nil {
		api.WriteMarshalError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, data)
}

// SelectTheme forwards a theme selection to the running theme pipeline container.
func (h *Handler) SelectTheme(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromRequest(r)
	storeID := chi.URLParam(r, "id")
	if err := h.requireThemeProject(r.Context(), user.ID, storeID); err != nil {
		writeProjectTypeError(w, err)
		return
	}
	var req ThemeSelection
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest, "Invalid request body", "VALIDATION_ERROR")
		return
	}
	if err := h.services.Theme.SelectTheme(r.Context(), storeID, req); err != nil {
		api.WriteInternalError(w, "Failed to select theme", err)
		return
	}
	api.WriteSuccessOK(w)
}

// decodeInputRequest decodes and validates the input field from the request body.
func decodeInputRequest(r *http.Request) (string, error) {
	var req struct {
		Input string `json:"input"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return "", err
	}
	if req.Input == "" {
		return "", errors.New("input is required")
	}
	return req.Input, nil
}

// ProvideInput sends user input (e.g. a password) to the running theme pipeline.
func (h *Handler) ProvideInput(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromRequest(r)
	storeID := chi.URLParam(r, "id")
	if err := h.requireThemeProject(r.Context(), user.ID, storeID); err != nil {
		writeProjectTypeError(w, err)
		return
	}
	input, err := decodeInputRequest(r)
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "Invalid request body", "VALIDATION_ERROR")
		return
	}
	if err := h.services.Theme.ProvideInput(r.Context(), storeID, input); err != nil {
		api.WriteInternalError(w, "Failed to provide input", err)
		return
	}
	api.WriteSuccessOK(w)
}

func (h *Handler) requireThemeProject(ctx context.Context, userID, storeID string) error {
	s, err := h.services.Stores.Get(ctx, userID, storeID)
	if err != nil {
		return err
	}
	if s.ProjectType != store.ProjectTypeTheme {
		return ErrInvalidProjectType
	}
	return nil
}

func writeProjectTypeError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrInvalidProjectType) {
		api.WriteError(w, http.StatusBadRequest, "This operation requires a theme project", "INVALID_PROJECT_TYPE")
		return
	}
	writeStoreError(w, err)
}

func writeStoreError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrStoreNotFound) {
		api.WriteError(w, http.StatusNotFound, "Store not found", "NOT_FOUND")
		return
	}
	api.WriteInternalError(w, "Failed to get store", err)
}
