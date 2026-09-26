package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/KennethhPoenadi/PixelCloud/api/internal/apierr"
	"github.com/KennethhPoenadi/PixelCloud/api/internal/db"
	"github.com/KennethhPoenadi/PixelCloud/api/internal/httpx"
)

type presetResponse struct {
	ID        uuid.UUID       `json:"id"`
	Name      string          `json:"name"`
	Pipeline  json.RawMessage `json:"pipeline"`
	IsSystem  bool            `json:"is_system"`
	CreatedAt time.Time       `json:"created_at"`
}

func toPresetResponse(p db.Preset) presetResponse {
	return presetResponse{ID: p.ID, Name: p.Name, Pipeline: p.Pipeline, IsSystem: p.IsSystem, CreatedAt: p.CreatedAt}
}

func (s *Server) listPresets(w http.ResponseWriter, r *http.Request) {
	p := mustPrincipal(r)
	ctx, cancel := context.WithTimeout(r.Context(), db.QueryTimeout)
	defer cancel()
	presets, err := s.store.ListPresets(ctx, p.UserID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	items := make([]presetResponse, 0, len(presets))
	for _, pr := range presets {
		items = append(items, toPresetResponse(pr))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

type createPresetRequest struct {
	Name     string          `json:"name"`
	Pipeline json.RawMessage `json:"pipeline"`
}

func (s *Server) createPreset(w http.ResponseWriter, r *http.Request) {
	p := mustPrincipal(r)
	var req createPresetRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	name := strings.TrimSpace(req.Name)
	if n := utf8.RuneCountInString(name); n < 1 || n > 50 {
		httpx.WriteError(w, r, apierr.New(apierr.Validation, "name must be 1-50 characters"))
		return
	}
	pl, err := validatePipeline(req.Pipeline)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), db.QueryTimeout)
	defer cancel()
	preset, err := s.store.CreatePreset(ctx, p.UserID, name, pl)
	switch {
	case errors.Is(err, db.ErrPresetExists):
		httpx.WriteError(w, r, apierr.New(apierr.Conflict, "you already have a preset named %q", name))
		return
	case errors.Is(err, db.ErrPresetLimit):
		httpx.WriteError(w, r, apierr.New(apierr.Validation, "you can save at most %d presets", db.MaxCustomPresets))
		return
	case err != nil:
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, toPresetResponse(preset))
}

func (s *Server) deletePreset(w http.ResponseWriter, r *http.Request) {
	p := mustPrincipal(r)
	id, err := parseID(r, "presetID")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), db.QueryTimeout)
	defer cancel()
	err = s.store.DeletePreset(ctx, p.UserID, id)
	if errors.Is(err, db.ErrNotFound) {
		httpx.WriteError(w, r, apierr.New(apierr.NotFound, "preset not found"))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
