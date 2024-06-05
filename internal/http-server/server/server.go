package server

import (
	"github.com/go-chi/chi/v5"
	"github.com/nglmq/gofermart-loyalty-programm/internal/config"
	"github.com/nglmq/gofermart-loyalty-programm/internal/http-server/handlers"
	"github.com/nglmq/gofermart-loyalty-programm/internal/http-server/handlers/balance"
	"github.com/nglmq/gofermart-loyalty-programm/internal/http-server/handlers/orders"
	"github.com/nglmq/gofermart-loyalty-programm/internal/middleware"
	"github.com/nglmq/gofermart-loyalty-programm/internal/middleware/logger"
	"github.com/nglmq/gofermart-loyalty-programm/internal/storage/postgres"
	"log/slog"
	"net/http"
)

func Start() (http.Handler, error) {
	config.ParseFlags()

	storage, err := postgres.New()
	if err != nil {
		slog.Error("failed to init db")
		return nil, err
	}

	r := chi.NewRouter()

	r.Use(logger.RequestLogger)
	r.Route("/api/user/", func(r chi.Router) {
		r.Post("/register", handlers.RegistrationHandle(storage))
		r.Post("/login", handlers.LoginHandle(storage))
		r.Post("/orders", orders.LoadOrderHandle(storage))
		r.Post("/balance/withdraw", balance.RequestWithdrawHandle(storage))
		r.Get("/orders", orders.GetOrdersHandle(storage))
		r.Get("/balance", balance.CheckBalanceHandle(storage))
		r.Get("/withdrawals", balance.GetWithdrawalsHandle(storage))
	})
	r.Group(func(r chi.Router) {
		r.Use(middleware.LimitMiddleware)
		r.Get("/api/user/orders/{number}", orders.GetOrderHandle(storage))
	})

	return r, nil
}
