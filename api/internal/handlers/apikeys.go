package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/KennethhPoenadi/PixelCloud/api/internal/apierr"
	"github.com/KennethhPoenadi/PixelCloud/api/internal/auth"
	"github.com/KennethhPoenadi/PixelCloud/api/internal/db"
	"github.com/KennethhPoenadi/PixelCloud/api/internal/httpx"
)

// requireAPIPlan allows the route only for plans with API access, and only
// with a browser session: API keys cannot mint or revoke other keys.
func (s *Server) requireAPIPlan(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := mustPrincipal(r)
		if p.ViaAPIKey {
			httpx.WriteError(w, r, apierr.New(apierr.Forbidden, "API keys cannot manage API keys; sign in instead"))
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), db.QueryTimeout)
		defer cancel()
		plan, err := s.store.PlanForUser(ctx, p.UserID)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		if !plan.APIAccess {
			httpx.WriteError(w, r, apierr.New(apierr.Forbidden, "API access requires the Business plan"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) listAPIKeys(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), db.QueryTimeout)
	defer cancel()
	keys, err := s.store.ListAPIKeys(ctx, mustPrincipal(r).UserID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": keys})
}

type createAPIKeyRequest struct {
	Label *string `json:"label"`
}

type createdAPIKey struct {
	db.APIKey
	Key string `json:"key"`
}

func (s *Server) createAPIKey(w http.ResponseWriter, r *http.Request) {
	var req createAPIKeyRequest
	if r.ContentLength != 0 {
		if err := httpx.DecodeJSON(r, &req); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	if req.Label != nil {
		l := strings.TrimSpace(*req.Label)
		if utf8.RuneCountInString(l) > 60 {
			httpx.WriteError(w, r, apierr.New(apierr.Validation, "label must be at most 60 characters"))
			return
		}
		if l == "" {
			req.Label = nil
		} else {
			req.Label = &l
		}
	}
	plain, hash, err := auth.NewAPIKey()
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), db.QueryTimeout)
	defer cancel()
	key, err := s.store.CreateAPIKey(ctx, mustPrincipal(r).UserID, req.Label, hash)
	if errors.Is(err, db.ErrNotFound) {
		httpx.WriteError(w, r, apierr.New(apierr.Validation, "at most %d active API keys are allowed", db.MaxAPIKeys))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, createdAPIKey{APIKey: key, Key: plain})
}

func (s *Server) revokeAPIKey(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "keyID")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), db.QueryTimeout)
	defer cancel()
	err = s.store.RevokeAPIKey(ctx, mustPrincipal(r).UserID, id)
	if errors.Is(err, db.ErrNotFound) {
		httpx.WriteError(w, r, apierr.New(apierr.NotFound, "API key not found"))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
