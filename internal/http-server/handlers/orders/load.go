package orders

import (
	"context"
	"errors"
	"fmt"
	"github.com/nglmq/gofermart-loyalty-programm/internal/auth"
	"github.com/nglmq/gofermart-loyalty-programm/internal/storage"
	"github.com/nglmq/gofermart-loyalty-programm/internal/validation"
	"io"
	"net/http"
	"strconv"
)

type OrderLoader interface {
	LoadOrder(ctx context.Context, login, orderID string) error
}

func LoadOrderHandle(orderLoader OrderLoader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "User not authorized", http.StatusUnauthorized)
			return
		}

		//authCookie, err := r.Cookie("User")
		//if err != nil {
		//	http.Error(w, "User not authorized", http.StatusUnauthorized)
		//	return
		//}

		//login := auth.GetUserID(authCookie.Value)
		login := auth.GetUserID(authHeader)
		fmt.Println(login)

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Error reading request body", http.StatusBadRequest)
			return
		}
		orderID := string(body)
		if orderID == "" {
			http.Error(w, "No order ID provided", http.StatusBadRequest)
			return
		}

		intOrderID, _ := strconv.Atoi(orderID)
		if !validation.Valid(intOrderID) {
			http.Error(w, "Invalid order ID", http.StatusUnprocessableEntity)
			return
		}

		err = orderLoader.LoadOrder(r.Context(), login, orderID)
		if err != nil {
			if errors.Is(err, storage.ErrOrderAlreadyLoadedByUser) {
				http.Error(w, "Order already loaded", http.StatusOK)
				return
			}
			if errors.Is(err, storage.ErrOrderAlreadyLoadedByAnotherUser) {
				http.Error(w, "Order already loaded by another user", http.StatusConflict)
				return
			}

			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("Order loaded"))
	}
}
