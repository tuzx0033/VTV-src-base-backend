package model

import "errors"

// Domain sentinel errors. Use cases translate these to xhttp.AppError.
var (
	ErrUserDeleted          = errors.New("user is deleted")
	ErrInvalidCredentials   = errors.New("invalid credentials")
	ErrAccountLocked        = errors.New("account is locked")
	ErrCurrentPasswordWrong = errors.New("current password is incorrect")
	ErrUsernameTaken        = errors.New("username already exists")
)
