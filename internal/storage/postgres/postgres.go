package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/nglmq/gofermart-loyalty-programm/internal/config"
	"github.com/nglmq/gofermart-loyalty-programm/internal/storage"
	"github.com/nglmq/gofermart-loyalty-programm/internal/validation"
	"log/slog"
)

type Storage struct {
	db *sql.DB
}

func New() (*Storage, error) {
	db, err := sql.Open("pgx", config.DataBaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS users(
    	id SERIAL PRIMARY KEY, 
    	login TEXT NOT NULL UNIQUE, 
    	password TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP);
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to create users table: %w", err)
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS orders(
	    id SERIAL PRIMARY KEY, 
    	user_login TEXT NOT NULL,
    	orderId TEXT NOT NULL,  
    	loaded_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	    FOREIGN KEY (user_login) REFERENCES users (login));
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to create orders table: %w", err)
	}

	return &Storage{db: db}, nil
}

//func (s *Storage) SaveUser(ctx context.Context, login, password string) error {
//	stmt, err := s.db.PrepareContext(ctx, "INSERT INTO users(login, password) VALUES ($1, $2)")
//	if err != nil {
//		return fmt.Errorf("%w", err)
//	}
//	defer stmt.Close()
//
//	_, err = stmt.ExecContext(ctx, login, password)
//	if err != nil {
//		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == pgerrcode.UniqueViolation {
//			slog.Error("user already exists", login)
//
//			return fmt.Errorf("%w", storage.LoginAlreadyExists)
//		} else {
//			fmt.Println("user unique)")
//		}
//
//		return fmt.Errorf("%w", err)
//	}
//
//	return nil
//}

func (s *Storage) SaveUser(ctx context.Context, login, password string) error {
	var exists bool

	err := s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE login = $1)", login).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check user existence: %w", err)
	}
	if exists {
		slog.Error("user already exists", login)
		return fmt.Errorf("%w", storage.LoginAlreadyExists)
	}

	stmt, err := s.db.PrepareContext(ctx, `INSERT INTO users(login, password) VALUES ($1, $2)`)
	if err != nil {
		return fmt.Errorf("failed to prepare insert statement: %w", err)
	}
	defer stmt.Close()

	password, err = validation.HashPassword(password)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	_, err = stmt.ExecContext(ctx, login, password)
	if err != nil {
		return fmt.Errorf("failed to insert user: %w", err)
	}

	return nil
}

func (s *Storage) GetUser(ctx context.Context, login, password string) (string, error) {
	var correctPassword string
	var userExists bool

	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE login  =  $1)`, login).Scan(&userExists)
	if err != nil {
		return "", storage.UserNotFound
	}

	err = s.db.QueryRowContext(ctx, `SELECT password FROM users WHERE login = $1`, login).Scan(&correctPassword)
	if err != nil {
		return "", fmt.Errorf("failed to prepare select statement: %w", err)
	}

	if !validation.CheckPassword(password, correctPassword) {
		return "", storage.IncorrectPassword
	}

	return login, nil
}

func (s *Storage) LoadOrder(ctx context.Context, login, orderId string) error {
	var loadByLogin string

	err := s.db.QueryRowContext(ctx, "SELECT user_login FROM orders WHERE orderId = $1", orderId).Scan(&loadByLogin)
	fmt.Printf("login: '%s'\n", loadByLogin)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to check order existence: %w", err)
	}

	if loadByLogin == login {
		// Заказ уже был загружен текущим пользователем
		slog.Info("order already loaded by this user", login)
		return storage.OrderAlreadyLoadedByUser
	} else if loadByLogin != "" {
		// Заказ уже был загружен другим пользователем
		slog.Info("order already loaded by another user", loadByLogin)
		return storage.OrderAlreadyLoadedByAnotherUser
	}

	stmt, err := s.db.PrepareContext(ctx, `INSERT INTO orders(user_login, orderId) VALUES ($1, $2)`)
	if err != nil {
		return fmt.Errorf("failed to prepare insert statement: %w", err)
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(ctx, login, orderId)
	if err != nil {
		return fmt.Errorf("failed to insert order: %w", err)
	}

	return nil
}
