package balance

import (
	"context"
	"encoding/json"
	"github.com/nglmq/gofermart-loyalty-programm/internal/auth"
	"github.com/nglmq/gofermart-loyalty-programm/internal/storage/postgres"
	"net/http"
)

type UserBalanceGetter interface {
	GetBalance(ctx context.Context, login string) (postgres.Balance, error)
}

func CheckBalanceHandle(balanceGetter UserBalanceGetter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "User not authorized", http.StatusUnauthorized)
			return
		}

		login := auth.GetUserID(authHeader)

		balance, err := balanceGetter.GetBalance(r.Context(), login)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		balanceJSON, err := json.Marshal(balance)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		w.Write(balanceJSON)
	}
}
