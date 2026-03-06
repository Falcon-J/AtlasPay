package auth

import (
	"encoding/json"
	"net/http"

	"github.com/atlaspay/platform/internal/common/errors"
	"github.com/atlaspay/platform/internal/common/response"
	"github.com/go-chi/chi/v5"
)

// Handler handles auth HTTP requests
type Handler struct {
	service *Service
}

// NewHandler creates a new auth handler
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Routes returns the auth routes
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Post("/register", h.Register)
	r.Post("/login", h.Login)
	r.Post("/refresh", h.RefreshTokens)
	r.Post("/logout", h.Logout)

	return r
}

// Register handles user registration
// @Summary Register a new user
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "Registration details"
// @Success 201 {object} AuthResponse
// @Failure 400 {object} errors.AppError
// @Failure 409 {object} errors.AppError
// @Router /auth/register [post]
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errors.WriteError(w, errors.ErrBadRequest.WithDetails("invalid request body"))
		return
	}

	// Basic validation
	if req.Email == "" || req.Password == "" || req.FirstName == "" || req.LastName == "" {
		errors.WriteError(w, errors.ErrBadRequest.WithDetails("all fields are required"))
		return
	}

	if len(req.Password) < 8 {
		errors.WriteError(w, errors.ErrBadRequest.WithDetails("password must be at least 8 characters"))
		return
	}

	resp, err := h.service.Register(r.Context(), &req)
	if err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			errors.WriteError(w, appErr)
			return
		}
		errors.WriteError(w, errors.ErrInternalServer)
		return
	}

	response.Created(w, resp)
}

// Login handles user authentication
// @Summary Login user
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Login credentials"
// @Success 200 {object} AuthResponse
// @Failure 401 {object} errors.AppError
// @Router /auth/login [post]
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errors.WriteError(w, errors.ErrBadRequest.WithDetails("invalid request body"))
		return
	}

	if req.Email == "" || req.Password == "" {
		errors.WriteError(w, errors.ErrBadRequest.WithDetails("email and password are required"))
		return
	}

	resp, err := h.service.Login(r.Context(), &req)
	if err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			errors.WriteError(w, appErr)
			return
		}
		errors.WriteError(w, errors.ErrInternalServer)
		return
	}

	response.OK(w, resp)
}

// RefreshTokens handles token refresh
// @Summary Refresh access token
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body RefreshRequest true "Refresh token"
// @Success 200 {object} AuthResponse
// @Failure 401 {object} errors.AppError
// @Router /auth/refresh [post]
func (h *Handler) RefreshTokens(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errors.WriteError(w, errors.ErrBadRequest.WithDetails("invalid request body"))
		return
	}

	if req.RefreshToken == "" {
		errors.WriteError(w, errors.ErrBadRequest.WithDetails("refresh_token is required"))
		return
	}

	resp, err := h.service.RefreshTokens(r.Context(), &req)
	if err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			errors.WriteError(w, appErr)
			return
		}
		errors.WriteError(w, errors.ErrInternalServer)
		return
	}

	response.OK(w, resp)
}

// Logout handles user logout
// @Summary Logout user
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body RefreshRequest true "Refresh token to revoke"
// @Success 204 "No Content"
// @Failure 400 {object} errors.AppError
// @Router /auth/logout [post]
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errors.WriteError(w, errors.ErrBadRequest.WithDetails("invalid request body"))
		return
	}

	if err := h.service.Logout(r.Context(), req.RefreshToken); err != nil {
		errors.WriteError(w, errors.ErrInternalServer)
		return
	}

	response.NoContent(w)
}
