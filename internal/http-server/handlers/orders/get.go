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
	GetUnfinishedOrders() ([]string, error)
	UpdateBalancePlus(ctx context.Context, amount float64, orderID string) error
	UpdateOrderStatus(ctx context.Context, orderID string, status string) error
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

func ActualiseOrderData(updater DataUpdater) error {
	url := config.AccrualSystemAddress + "/api/orders/"

	orders, err := updater.GetUnfinishedOrders()
	if err != nil {
		return fmt.Errorf("error getting unfinished orders: %w", err)
	}

	for _, orderID := range orders {
		order, err := updateOrderData(url, orderID)
		if err != nil {
			slog.Info("Error processing order: ", orderID, err)
			continue
		}

		if err := updater.UpdateBalancePlus(context.Background(), order.Accrual, orderID); err != nil {
			slog.Info("Error updating balance for order: ", orderID, err)
			continue
		}
		if err := updater.UpdateOrderStatus(context.Background(), order.Number, orderID); err != nil {
			slog.Info("Error updating order status for order: ", orderID, err)
			continue
		}
	}

	return nil
}

func updateOrderData(baseUrl, orderID string) (Order, error) {
	var order Order
	url := baseUrl + orderID
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return order, fmt.Errorf("error creating request for order data: %w", err)
	}

	client := http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return Order{}, fmt.Errorf("error sending request for order data: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusTooManyRequests {
		return Order{}, storage.ErrTooManyRequests
	}
	if res.StatusCode != http.StatusNoContent {
		return Order{}, storage.ErrOrderNotFound
	}
	if res.StatusCode == http.StatusInternalServerError {
		return Order{}, fmt.Errorf("accrual server error: %w", err)
	}

	if res.StatusCode == http.StatusOK {
		if err := json.NewDecoder(res.Body).Decode(&order); err != nil {
			return Order{}, fmt.Errorf("error decoding order: %w", err)
		}
		body, _ := io.ReadAll(res.Body)
		slog.Info(string(body))
	}

	return order, nil
}
