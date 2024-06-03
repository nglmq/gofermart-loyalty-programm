package balance

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/nglmq/gofermart-loyalty-programm/internal/auth"
	"github.com/nglmq/gofermart-loyalty-programm/internal/storage"
	"github.com/nglmq/gofermart-loyalty-programm/internal/storage/postgres"
	"github.com/nglmq/gofermart-loyalty-programm/internal/validation"
	"io"
	"net/http"
	"strconv"
)

type WithdrawalRequest struct {
	Order string  `json:"order"`
	Sum   float64 `json:"sum"`
}

type UserBalanceWithdraw interface {
	RequestWithdraw(ctx context.Context, login string, amount float64, orderID string) error
	GetWithdrawals(ctx context.Context, login string) ([]postgres.Withdrawals, error)
}

func RequestWithdrawHandle(withdraw UserBalanceWithdraw) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "User not authorized", http.StatusUnauthorized)
			return
		}

		login := auth.GetUserID(authHeader)

		var withdrawalReq WithdrawalRequest

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Error reading request body", http.StatusInternalServerError)
			return
		}
		err = json.Unmarshal(body, &withdrawalReq)
		if err != nil {
			http.Error(w, "Error parsing request body", http.StatusBadRequest)
			return
		}

		intOrderID, _ := strconv.Atoi(withdrawalReq.Order)
		if !validation.Valid(intOrderID) {
			http.Error(w, "Invalid order ID", http.StatusUnprocessableEntity)
			return
		}

		err = withdraw.RequestWithdraw(r.Context(), login, withdrawalReq.Sum, withdrawalReq.Order)
		if err != nil {
			if errors.Is(err, storage.ErrNotEnoughBalance) {
				http.Error(w, "Not enough balance", http.StatusPaymentRequired)
				return
			}

			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Withdrawal requested"))
	}
}

func GetWithdrawalsHandle(withdraw UserBalanceWithdraw) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "User not authorized", http.StatusUnauthorized)
			return
		}

		login := auth.GetUserID(authHeader)

		withdrawals, err := withdraw.GetWithdrawals(r.Context(), login)
		if err != nil {
			if errors.Is(err, storage.ErrNoWithdrawalsFound) {
				http.Error(w, "No withdrawals found", http.StatusNoContent)
				return
			}

			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		withdrawalsJSON, err := json.Marshal(withdrawals)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		w.Write(withdrawalsJSON)
	}
}
