package orders

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/nglmq/gofermart-loyalty-programm/internal/auth"
	"github.com/nglmq/gofermart-loyalty-programm/internal/config"
	"github.com/nglmq/gofermart-loyalty-programm/internal/storage"
	"github.com/nglmq/gofermart-loyalty-programm/internal/storage/postgres"
	"io"
	"log/slog"
	"net/http"
)

type Order struct {
	Number  string  `json:"number"`
	Status  string  `json:"status"`
	Accrual float64 `json:"accrual,omitempty"`
}

type OrderGetter interface {
	// GetOrders GetOrder(ctx context.Context, orderID string) (postgres.Order, error)
	GetOrders(ctx context.Context, login string) ([]postgres.Order, error)
}

type DataUpdater interface {
	UpdateBalancePlus(ctx context.Context, amount float64, orderID string) error
	UpdateOrderStatus(ctx context.Context, orderID string, status string) error
}

func GetOrdersHandle(updater DataUpdater, orderGetter OrderGetter) http.HandlerFunc {
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
		if err != nil {
			http.Error(w, "Error getting orders: ", http.StatusInternalServerError)
			return
		}

		for _, order := range orders {
			err := ActualiseOrderData(updater, order.Number)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		orders, err = orderGetter.GetOrders(r.Context(), login)
		if err != nil {
			http.Error(w, "Error getting orders: ", http.StatusInternalServerError)
			return
		}

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

//func GetOrderHandle(dataUpdater DataUpdater) http.HandlerFunc {
//	url := "http://" + config.AccrualSystemAddress + "/api/user/orders/"
//
//	return func(w http.ResponseWriter, r *http.Request) {
//		var b []byte
//		var order Order
//
//		resp, err := http.NewRequest("GET", url, bytes.NewBuffer(b))
//		if err != nil {
//			http.Error(w, "Invalid request", http.StatusBadRequest)
//			return
//		}
//
//		err := json.NewDecoder(r.Body).Decode(&order)
//		fmt.Println(order)
//		if err != nil {
//			slog.Info("error decoding order:", err)
//			http.Error(w, "Invalid request", http.StatusBadRequest)
//			return
//		}
//
//		if err := dataUpdater.UpdateBalancePlus(r.Context(), order.Accrual, order.Number); err != nil {
//			slog.Info("error updating balance:", err)
//
//			http.Error(w, err.Error(), http.StatusInternalServerError)
//			return
//		}
//		if err := dataUpdater.UpdateOrderStatus(r.Context(), orderID, order.Status); err != nil {
//			slog.Info("error updating order status:", err)
//
//			http.Error(w, err.Error(), http.StatusInternalServerError)
//			return
//		}
//
//	}
//}

func ActualiseOrderData(updater DataUpdater, orderID string) error {
	url := "http://" + config.AccrualSystemAddress + "/api/orders/" + orderID
	var order Order

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("error actualising order data: %w", err)
	}

	client := http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending req: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusTooManyRequests {
		return storage.ErrTooManyRequests
	}
	if res.StatusCode != http.StatusNoContent {
		return storage.ErrOrderNotFound
	}
	if res.StatusCode == http.StatusInternalServerError {
		return fmt.Errorf("accrual server error: %w", err)
	}

	if res.StatusCode == http.StatusOK {
		if err := json.NewDecoder(res.Body).Decode(&order); err != nil {
			return fmt.Errorf("error decoding order: %w", err)
		}

		body, _ := io.ReadAll(res.Body)
		slog.Info(string(body))

		if err := updater.UpdateBalancePlus(req.Context(), order.Accrual, orderID); err != nil {
			return fmt.Errorf("error updating balance: %w", err)
		}
		if err := updater.UpdateOrderStatus(req.Context(), order.Number, orderID); err != nil {
			return fmt.Errorf("error updating order status: %w", err)
		}
	}

	return nil
}
