package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/KennethhPoenadi/PixelCloud/api/internal/apierr"
	"github.com/KennethhPoenadi/PixelCloud/api/internal/auth"
	"github.com/KennethhPoenadi/PixelCloud/api/internal/db"
	"github.com/KennethhPoenadi/PixelCloud/api/internal/httpx"
	"github.com/KennethhPoenadi/PixelCloud/api/internal/quota"
)

type registerRequest struct {
	Email       string  `json:"email"`
	Password    string  `json:"password"`
	DisplayName *string `json:"display_name"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type sessionResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	User      db.User   `json:"user"`
}

func normalizeEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email || len(email) > 254 {
		return "", apierr.New(apierr.Validation, "email is not valid")
	}
	return email, nil
}

func validatePassword(p string) error {
	// bcrypt only uses the first 72 bytes, so longer passwords are rejected outright.
	if utf8.RuneCountInString(p) < 8 || len(p) > 72 {
		return apierr.New(apierr.Validation, "password must be 8-72 characters")
	}
	return nil
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	email, err := normalizeEmail(req.Email)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := validatePassword(req.Password); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if req.DisplayName != nil {
		name := strings.TrimSpace(*req.DisplayName)
		if utf8.RuneCountInString(name) > 60 {
			httpx.WriteError(w, r, apierr.New(apierr.Validation, "display_name must be at most 60 characters"))
			return
		}
		if name == "" {
			req.DisplayName = nil
		} else {
			req.DisplayName = &name
		}
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), db.QueryTimeout)
	defer cancel()
	user, err := s.store.CreateUser(ctx, email, hash, req.DisplayName)
	if errors.Is(err, db.ErrEmailTaken) {
		httpx.WriteError(w, r, apierr.New(apierr.Conflict, "an account with this email already exists"))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	s.writeSession(w, r, http.StatusCreated, user)
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), db.QueryTimeout)
	defer cancel()
	user, err := s.store.UserByEmail(ctx, strings.ToLower(strings.TrimSpace(req.Email)))
	if err != nil && !errors.Is(err, db.ErrNotFound) {
		httpx.WriteError(w, r, err)
		return
	}
	// CheckPassword with an empty hash still spends bcrypt time (no enumeration).
	if !auth.CheckPassword(user.PasswordHash, req.Password) {
		httpx.WriteError(w, r, apierr.New(apierr.Unauthorized, "email or password is incorrect"))
		return
	}
	s.writeSession(w, r, http.StatusOK, user)
}

func (s *Server) writeSession(w http.ResponseWriter, r *http.Request, status int, user db.User) {
	token, exp, err := s.tokens.Issue(user.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, status, sessionResponse{Token: token, ExpiresAt: exp, User: user})
}

type meResponse struct {
	User  db.User  `json:"user"`
	Plan  db.Plan  `json:"plan"`
	Usage db.Usage `json:"usage"`
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	p := mustPrincipal(r)
	ctx, cancel := context.WithTimeout(r.Context(), db.QueryTimeout)
	defer cancel()
	user, plan, err := s.store.UserWithPlan(ctx, p.UserID)
	if errors.Is(err, db.ErrNotFound) {
		httpx.WriteError(w, r, apierr.New(apierr.Unauthorized, "account no longer exists"))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	usage, err := s.store.UsageForPeriod(ctx, p.UserID, quota.PeriodStart(time.Now()))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, meResponse{User: user, Plan: plan, Usage: usage})
}

func (s *Server) listPlans(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), db.QueryTimeout)
	defer cancel()
	plans, err := s.store.ListPlans(ctx)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": plans})
}

// authenticate accepts a Bearer JWT, or an X-API-Key on plans with API access,
// and stores the Principal in the context.
func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if key := r.Header.Get("X-API-Key"); key != "" {
			ctx, cancel := context.WithTimeout(r.Context(), db.QueryTimeout)
			userID, allowed, err := s.store.UserByAPIKey(ctx, auth.HashAPIKey(key))
			cancel()
			switch {
			case errors.Is(err, db.ErrNotFound):
				httpx.WriteError(w, r, apierr.New(apierr.Unauthorized, "API key is invalid or revoked"))
			case err != nil:
				httpx.WriteError(w, r, err)
			case !allowed:
				httpx.WriteError(w, r, apierr.New(apierr.Forbidden, "API access requires the Business plan"))
			default:
				s.serveAs(w, r, next, auth.Principal{UserID: userID, ViaAPIKey: true})
			}
			return
		}

		header := r.Header.Get("Authorization")
		token, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || token == "" {
			httpx.WriteError(w, r, apierr.New(apierr.Unauthorized, "missing or malformed Authorization header"))
			return
		}
		userID, err := s.tokens.Parse(token)
		if err != nil {
			httpx.WriteError(w, r, apierr.New(apierr.Unauthorized, "session expired or invalid, please log in again"))
			return
		}
		s.serveAs(w, r, next, auth.Principal{UserID: userID})
	})
}

func (s *Server) serveAs(w http.ResponseWriter, r *http.Request, next http.Handler, p auth.Principal) {
	ctx := auth.WithPrincipal(r.Context(), p)
	ctx = httpx.WithLogger(ctx, httpx.Logger(ctx).With("user_id", p.UserID.String()))
	next.ServeHTTP(w, r.WithContext(ctx))
}

// mustPrincipal is only called behind authenticate, which guarantees presence.
func mustPrincipal(r *http.Request) auth.Principal {
	p, _ := auth.FromContext(r.Context())
	return p
}
