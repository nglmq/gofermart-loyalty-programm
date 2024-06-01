package user_auth

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/go-playground/validator/v10"
	"github.com/nglmq/gofermart-loyalty-programm/internal/auth"
	"github.com/nglmq/gofermart-loyalty-programm/internal/storage"
	"log/slog"
	"net/http"
)

type LoginData struct {
	Login    string `json:"login" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type UserGetter interface {
	GetUser(ctx context.Context, login, password string) (string, error)
}

func LoginHandle(userGetter UserGetter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method is not allowed", http.StatusMethodNotAllowed)
			return
		}

		var data LoginData

		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			slog.Info("invalid login request", err)

			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := validator.New().Struct(data); err != nil {
			slog.Error("invalid validation for login", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		_, err := userGetter.GetUser(r.Context(), data.Login, data.Password)
		if errors.Is(err, storage.IncorrectPassword) {
			slog.Info("password is incorrect")

			http.Error(w, "password is incorrect", http.StatusUnauthorized)
			return
		}
		if err != nil {
			if errors.Is(err, storage.UserNotFound) {
				slog.Info("user not found")

				http.Error(w, "user not found", http.StatusUnauthorized)
				return
			}
			slog.Info("failed to get user while login", err)

			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		tokenString, err := auth.BuildJWTString(data.Login)
		if err != nil {
			slog.Info("failed to create JWT token", err)
			http.Error(w, "failed to create JWT token", http.StatusInternalServerError)
			return
		}

		//w.Header().Set("Authorization", tokenString)
		http.SetCookie(w, &http.Cookie{
			Name:     "User",
			Value:    tokenString,
			Path:     "/",
			HttpOnly: true,
			Secure:   true,
		})

		w.Write([]byte(data.Login))
		w.WriteHeader(http.StatusOK)
	}
}
