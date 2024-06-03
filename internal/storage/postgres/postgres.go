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
	"time"
)

type Storage struct {
	db *sql.DB
}

type Order struct {
	Number     string    `json:"number" db:"orderId"`
	Status     string    `json:"status" db:"status"`
	Accrual    string    `json:"accrual,omitempty" db:"accrual"`
	UploadedAt time.Time `json:"uploaded_at" db:"uploaded_at"`
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
    	status TEXT NOT NULL DEFAULT 'NEW',
    	accrual TEXT,
    	uploaded_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	    FOREIGN KEY (user_login) REFERENCES users (login));
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to create orders table: %w", err)
	}

	return &Storage{db: db}, nil
}

func (s *Storage) SaveUser(ctx context.Context, login, password string) error {
	var exists bool

	err := s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE login = $1)", login).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check user existence: %w", err)
	}
	if exists {
		slog.Error("user already exists", login)
		return fmt.Errorf("%w", storage.ErrLoginAlreadyExists)
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
		return "", storage.ErrUserNotFound
	}

	err = s.db.QueryRowContext(ctx, `SELECT password FROM users WHERE login = $1`, login).Scan(&correctPassword)
	if err != nil {
		return "", fmt.Errorf("failed to prepare select statement: %w", err)
	}

	if !validation.CheckPassword(password, correctPassword) {
		return "", storage.ErrIncorrectPassword
	}

	return login, nil
}

func (s *Storage) LoadOrder(ctx context.Context, login, orderID string) error {
	var loadByLogin string

	err := s.db.QueryRowContext(ctx, "SELECT user_login FROM orders WHERE orderId = $1", orderID).Scan(&loadByLogin)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to check order existence: %w", err)
	}

	if loadByLogin == login {
		slog.Info("order already loaded by this user", login)
		return storage.ErrOrderAlreadyLoadedByUser
	} else if loadByLogin != "" {
		slog.Info("order already loaded by another user", loadByLogin)
		return storage.ErrOrderAlreadyLoadedByAnotherUser
	}

	stmt, err := s.db.PrepareContext(ctx, `INSERT INTO orders(user_login, orderId) VALUES ($1, $2)`)
	if err != nil {
		return fmt.Errorf("failed to prepare insert statement: %w", err)
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(ctx, login, orderID)
	if err != nil {
		return fmt.Errorf("failed to insert order: %w", err)
	}

	return nil
}

func (s *Storage) GetOrders(ctx context.Context, login string) ([]Order, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT orderId, status, accrual, uploaded_at FROM orders WHERE user_login = $1 ORDER BY uploaded_at ASC", login)
	if err != nil {
		return nil, fmt.Errorf("failed to query orders: %w", err)
	}
	defer rows.Close()

	var orders []Order

	for rows.Next() {
		var order Order
		var accrual sql.NullString

		if err := rows.Scan(&order.Number, &order.Status, &accrual, &order.UploadedAt); err != nil {
			return nil, fmt.Errorf("failed to scan order: %w", err)
		}

		if accrual.Valid {
			order.Accrual = accrual.String
		}

		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error occurred during row iteration: %w", err)
	}

	if len(orders) == 0 {
		return nil, storage.ErrNoOrders
	}

	return orders, nil
}
