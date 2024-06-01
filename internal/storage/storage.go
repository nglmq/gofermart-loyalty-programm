package storage

import "errors"

var (
	LoginAlreadyExists              = errors.New("login	already exists")
	UserNotFound                    = errors.New("user not found")
	IncorrectPassword               = errors.New("incorrect password")
	OrderAlreadyLoadedByUser        = errors.New("order already loaded")
	OrderAlreadyLoadedByAnotherUser = errors.New("order already loaded by another user")
)
