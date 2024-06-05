package storage

import "errors"

var (
	ErrLoginAlreadyExists              = errors.New("login	already exists")
	ErrUserNotFound                    = errors.New("user not found")
	ErrIncorrectPassword               = errors.New("incorrect password")
	ErrOrderAlreadyLoadedByUser        = errors.New("order already loaded")
	ErrOrderAlreadyLoadedByAnotherUser = errors.New("order already loaded by another user")
	ErrNoOrders                        = errors.New("no orders found")
	ErrOrderNotFound                   = errors.New("order not found")
	ErrNotEnoughBalance                = errors.New("not enough balance")
	ErrNoWithdrawalsFound              = errors.New("no withdrawals found")
)
