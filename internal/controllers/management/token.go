package management

import (
	"github.com/google/uuid"
)

// generateAuthToken returns a standard UUID string with hyphens.
func generateAuthToken() string {
	return uuid.New().String()
}
