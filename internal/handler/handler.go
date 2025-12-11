package handler

import (
	"net/http"

	"fsanano/go-test/internal/service/skinport"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Handler struct {
	router         *chi.Mux
	skinportClient *skinport.Client
	shopHandler    *ShopHandler
}

func NewHandler(skinportClient *skinport.Client, shopHandler *ShopHandler) *Handler {
	router := chi.NewRouter()

	// Middleware
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(middleware.RequestID)

	h := &Handler{
		router:         router,
		skinportClient: skinportClient,
		shopHandler:    shopHandler,
	}

	h.registerRoutes()
	return h
}

func (h *Handler) registerRoutes() {
	h.router.Route("/v1", func(r chi.Router) {
		r.Get("/health", h.HealthCheck)

		r.Route("/skinport", func(r chi.Router) {
			r.Get("/items", h.GetSkinportItems)
		})

		r.Post("/buy", h.shopHandler.BuyItem)
	})
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.router.ServeHTTP(w, r)
}

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
