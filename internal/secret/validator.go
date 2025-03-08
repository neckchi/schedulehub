package env

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
)

const (
	maxKeyLength   = 100000
	maxValueLength = 1000000
)

func validateFilePath(path string) error {
	// Check for path traversal attempts
	if strings.Contains(path, "..") {
		return ErrInvalidPath
	}

	// Ensure the path is absolute or relative to current directory
	cleanPath := filepath.Clean(path)
	if filepath.IsAbs(cleanPath) || !strings.HasPrefix(cleanPath, "..") {
		return nil
	}

	return ErrInvalidPath
}

func validateKeyValue(key, value string) error {
	if err := validateKey(key); err != nil {
		return err
	}
	return validateValue(value)
}

func validateKey(key string) error {
	if len(key) == 0 {
		return ErrEmptyKey
	}

	if len(key) > maxKeyLength {
		return fmt.Errorf("%w: maximum length is %d", ErrInvalidKey, maxKeyLength)
	}

	for i, char := range key {
		if i == 0 && !unicode.IsLetter(char) && char != '_' {
			return fmt.Errorf("%w: must start with letter or underscore", ErrInvalidKey)
		}
		if !unicode.IsLetter(char) && !unicode.IsNumber(char) && char != '_' {
			return fmt.Errorf("%w: invalid character %q", ErrInvalidKey, char)
		}
	}

	return nil
}

func validateValue(value string) error {
	if len(value) > maxValueLength {
		return fmt.Errorf("%w: maximum length is %d", ErrInvalidValue, maxValueLength)
	}

	// Add additional value validation as needed
	return nil
}
