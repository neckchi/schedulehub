package env

import "errors"

var (
	ErrInvalidPath   = errors.New("invalid file path")
	ErrInvalidFormat = errors.New("invalid line format")
	ErrEmptyKey      = errors.New("empty key not allowed")
	ErrInvalidKey    = errors.New("invalid key")
	ErrInvalidValue  = errors.New("invalid value")
)
