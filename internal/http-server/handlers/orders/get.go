package orders

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/nglmq/gofermart-loyalty-programm/internal/auth"
	"github.com/nglmq/gofermart-loyalty-programm/internal/storage"
	"github.com/nglmq/gofermart-loyalty-programm/internal/storage/postgres"
	"log/slog"
	"net/http"
)

type OrderGetter interface {
	GetOrders(ctx context.Context, login string) ([]postgres.Order, error)
}

func GetOrdersHandle(orderGetter OrderGetter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		slog.Info(authHeader)
		if authHeader == "" {
			http.Error(w, "User not authorized", http.StatusUnauthorized)
			return
		}

		login := auth.GetUserID(authHeader)
		slog.Info(login + "LOGIN FOR GETTING ORDERS")

		orders, err := orderGetter.GetOrders(r.Context(), login)
		slog.Info("len of orders slice:", len(orders))
		if err != nil {
			if errors.Is(err, storage.ErrNoOrders) {
				http.Error(w, "No orders found", http.StatusNoContent)
				return
			}

			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		response, err := json.Marshal(orders)
		slog.Info(string(response))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(response)

		slog.Info("getting orders done")

	}
}
