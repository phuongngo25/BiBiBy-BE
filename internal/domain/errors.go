package domain

import "errors"

// Sentinel errors shared across all domain packages.
var (
	ErrFoodNotFound        = errors.New("food not found")
	ErrUserNotFound        = errors.New("user not found")
	ErrUserAlreadyExists   = errors.New("user already exists")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrInternalServerError = errors.New("internal server error")
)
