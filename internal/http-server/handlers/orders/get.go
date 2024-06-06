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
	Number  string  `json:"order"`
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
	UpdateOrderStatus(ctx context.Context, accrual float64, status, orderID string) error
}

func GetOrdersHandle(orderGetter OrderGetter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")

		if authHeader == "" {
			http.Error(w, "User not authorized", http.StatusUnauthorized)
			return
		}

		login := auth.GetUserID(authHeader)

		orders, err := orderGetter.GetOrders(r.Context(), login)
		if err != nil {
			if errors.Is(err, storage.ErrNoOrders) {
				http.Error(w, "No orders found", http.StatusNoContent)
				return
			}
			http.Error(w, "Error getting orders", http.StatusInternalServerError)
			return
		}

		response, err := json.Marshal(orders)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(response)
	}
}

func ActualiseOrderData(updater DataUpdater) error {
	url := config.AccrualSystemAddress + "/api/orders/"

	orders, err := updater.GetUnfinishedOrders()
	if err != nil {
		return fmt.Errorf("error getting unfinished orders: %w", err)
	}

	slog.Info("Processing orders: ", orders)
	for _, orderID := range orders {
		order, err := updateOrderData(url, orderID)
		if err != nil {
			continue
		}
		if err := updater.UpdateBalancePlus(context.Background(), order.Accrual, order.Number); err != nil {
			return err
		}

		if err := updater.UpdateOrderStatus(context.Background(), order.Accrual, order.Status, order.Number); err != nil {
			return err
		}
	}

	return nil
}

func updateOrderData(baseURL, orderID string) (Order, error) {
	var order Order
	url := baseURL + orderID

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

	switch res.StatusCode {
	case http.StatusTooManyRequests:
		return Order{}, storage.ErrTooManyRequests
	case http.StatusNoContent:
		return Order{}, storage.ErrOrderNotFound
	case http.StatusInternalServerError:
		return Order{}, fmt.Errorf("accrual server error: %w", err)
	}

	if res.StatusCode == http.StatusOK {
		body, _ := io.ReadAll(res.Body)

		if err := json.Unmarshal(body, &order); err != nil {
			return Order{}, fmt.Errorf("error decoding order: %w", err)
		}
	}

	return order, nil
}
