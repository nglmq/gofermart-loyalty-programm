package handlers

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

type RegistrationData struct {
	Login    string `json:"login" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type UserSaver interface {
	SaveUser(ctx context.Context, login, password string) error
}

func RegistrationHandle(userSaver UserSaver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			slog.Info("invalid method", r.Method)
			http.Error(w, "Method is not allowed", http.StatusMethodNotAllowed)
			return
		}

		var data RegistrationData

		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			slog.Info("invalid register request", err)

			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := validator.New().Struct(data); err != nil {
			slog.Info("invalid validation for reg", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		err := userSaver.SaveUser(r.Context(), data.Login, data.Password)
		if errors.Is(err, storage.ErrLoginAlreadyExists) {
			slog.Info("user already exists", data.Login)

			http.Error(w, "user already exists", http.StatusConflict)
			return
		}
		if err != nil {
			slog.Info("failed to save user while reg", err)

			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		slog.Info("user saved")
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
