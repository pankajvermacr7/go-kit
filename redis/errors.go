package redis

import "fmt"

// WrapError provides additional context to errors.
func WrapError(action, key string, err error) error {
	return fmt.Errorf("error while %s for key '%s': %w", action, key, err)
}
