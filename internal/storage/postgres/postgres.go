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
	Accrual    float64   `json:"accrual,omitempty" db:"accrual"`
	UploadedAt time.Time `json:"uploaded_at" db:"uploaded_at"`
}

type Balance struct {
	Current   float64 `json:"current" db:"current_balance"`
	Withdrawn float64 `json:"withdrawn" db:"withdrawn"`
}

type Withdrawals struct {
	OrderID     string    `json:"order" db:"orderId"`
	Sum         float64   `json:"sum" db:"amount"`
	ProcessedAt time.Time `json:"processed_at" db:"processed_at"`
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
    	orderId TEXT NOT NULL UNIQUE,
    	status TEXT NOT NULL DEFAULT 'NEW',
    	accrual FLOAT,
    	uploaded_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	    FOREIGN KEY (user_login) REFERENCES users (login));
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to create orders table: %w", err)
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS balances(
	    id SERIAL PRIMARY KEY, 
    	user_login TEXT NOT NULL,
    	current_balance FLOAT NOT NULL DEFAULT 0 CHECK(current_balance >= 0),
    	withdrawn FLOAT NOT NULL,
	    FOREIGN KEY (user_login) REFERENCES users (login));
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to create balances table: %w", err)
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS withdrawals(
	    id SERIAL PRIMARY KEY, 
    	user_login TEXT NOT NULL,
    	orderId TEXT NOT NULL,
    	amount FLOAT NOT NULL,
    	processed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	    FOREIGN KEY (user_login) REFERENCES users (login),
	    FOREIGN KEY (orderId) REFERENCES orders (orderId));
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to create withdrawals table: %w", err)
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
		return []Order{}, fmt.Errorf("failed to query orders: %w", err)
	}
	defer rows.Close()

	var orders []Order

	for rows.Next() {
		var order Order
		var accrual sql.NullFloat64

		if err := rows.Scan(&order.Number, &order.Status, &accrual, &order.UploadedAt); err != nil {
			return []Order{}, fmt.Errorf("failed to scan order: %w", err)
		}
		if accrual.Valid {
			order.Accrual = accrual.Float64
		}

		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		return []Order{}, fmt.Errorf("error occurred during row iteration: %w", err)
	}

	if len(orders) == 0 {
		return []Order{}, storage.ErrNoOrders
	}

	return orders, nil
}

//func (s *Storage) GetOrder(ctx context.Context, orderID string) (Order, error) {
//	var order Order
//	var accrual sql.NullFloat64
//
//	err := s.db.QueryRowContext(ctx, "SELECT orderID, status, accrual FROM orders WHERE orderID = $1", orderID).Scan(&order.Number, &order.Status, &accrual)
//	if err != nil {
//		if errors.Is(err, sql.ErrNoRows) {
//			return Order{}, storage.ErrOrderNotFound
//		}
//		return Order{}, fmt.Errorf("failed to query order: %w", err)
//	}
//
//	if accrual.Valid {
//		order.Accrual = accrual.Float64
//	}
//
//	return order, nil
//}

func (s *Storage) GetBalance(ctx context.Context, login string) (Balance, error) {
	var balance Balance

	err := s.db.QueryRowContext(ctx, "SELECT current_balance, withdrawn FROM balances WHERE user_login = $1", login).Scan(&balance.Current, &balance.Withdrawn)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Balance{}, fmt.Errorf("failed to query balance: %w", err)
	}

	return balance, nil
}

func (s *Storage) UpdateBalanceMinus(ctx context.Context, login string, amount float64) error {
	stmt, err := s.db.PrepareContext(ctx, `UPDATE balances SET current_balance = current_balance - $1, withdrawn  =  withdrawn  +  $1 WHERE user_login  =  $2`)
	if err != nil {
		return fmt.Errorf("failed to prepare update statement: %w", err)
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(ctx, amount, login)
	if err != nil {
		return fmt.Errorf("failed to update balance: %w", err)
	}

	return nil
}

func (s *Storage) UpdateBalancePlus(ctx context.Context, amount float64, orderID string) error {
	var login string

	err := s.db.QueryRowContext(ctx, "SELECT user_login FROM orders WHERE orderID  =  $1", orderID).Scan(&login)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to query order: %w", err)
	}

	stmt, err := s.db.PrepareContext(ctx, `UPDATE balances SET current_balance = current_balance + $1 WHERE user_login = $2`)
	if err != nil {
		return fmt.Errorf("failed to prepare update statement: %w", err)
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(ctx, amount, login)
	if err != nil {
		return fmt.Errorf("failed to update balance: %w", err)
	}

	return nil
}

func (s *Storage) UpdateOrderStatus(ctx context.Context, orderID string, status string) error {
	stmt, err := s.db.PrepareContext(ctx, `UPDATE orders SET status = $1 WHERE orderID = $2`)
	if err != nil {
		return fmt.Errorf("failed to prepare update statement: %w", err)
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(ctx, status, orderID)
	if err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	return nil
}

func (s *Storage) RequestWithdraw(ctx context.Context, login string, amount float64, orderID string) error {
	var balance float64

	err := s.db.QueryRowContext(ctx, "SELECT current_balance FROM balances WHERE user_login  =  $1", login).Scan(&balance)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to query balance: %w", err)
	}
	if balance < amount {
		return storage.ErrNotEnoughBalance
	}

	stmt, err := s.db.PrepareContext(ctx, `INSERT INTO withdrawals(user_login, amount, orderId) VALUES  ($1,  $2,  $3)`)
	if err != nil {
		return fmt.Errorf("failed to prepare insert statement: %w", err)
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(ctx, login, amount, orderID)
	if err != nil {
		return fmt.Errorf("failed to insert withdrawal: %w", err)
	}

	err = s.UpdateBalanceMinus(ctx, login, amount)
	if err != nil {
		return fmt.Errorf("failed to update balance: %w", err)
	}

	return nil
}

func (s *Storage) GetWithdrawals(ctx context.Context, login string) ([]Withdrawals, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT orderId, amount, processed_at FROM withdrawals WHERE user_login = $1 ORDER BY processed_at ASC`, login)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return []Withdrawals{}, fmt.Errorf("failed to query withdrawals: %w", err)
	}
	defer rows.Close()

	var withdrawals []Withdrawals

	for rows.Next() {
		var withdrawal Withdrawals

		if err := rows.Scan(&withdrawal.OrderID, &withdrawal.Sum, &withdrawal.ProcessedAt); err != nil {
			return []Withdrawals{}, fmt.Errorf("failed to scan withdrawal: %w", err)
		}

		withdrawals = append(withdrawals, withdrawal)
	}

	if len(withdrawals) == 0 {
		return []Withdrawals{}, storage.ErrNoWithdrawalsFound
	}

	return withdrawals, nil
}
