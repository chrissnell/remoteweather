package provision

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

const (
	// PasswordLength is the default length for generated passwords
	PasswordLength = 24
	// Charset for password generation
	passwordCharset = "abcdefghijklmnopqrstuvwxyz" +
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
		"0123456789" +
		"!@#$%^&*()-_=+[]{}|;:,.<>?"
)

// GeneratePassword generates a cryptographically secure random password
func GeneratePassword(length int) (string, error) {
	if length <= 0 {
		length = PasswordLength
	}

	password := make([]byte, length)
	charsetLen := big.NewInt(int64(len(passwordCharset)))

	for i := range password {
		num, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			return "", fmt.Errorf("failed to generate random number: %w", err)
		}
		password[i] = passwordCharset[num.Int64()]
	}

	return string(password), nil
}

// DisplayPasswordWarning prints a prominent warning with the generated password
func DisplayPasswordWarning(password string) {
	fmt.Println()
	fmt.Println("🔐 Generated Password")
	fmt.Println("╔════════════════════════════════════════════════╗")
	fmt.Println("║  ⚠️  SAVE THIS PASSWORD - IT WON'T BE SHOWN AGAIN  ║")
	fmt.Println("╚════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("  Password: %s\n", password)
	fmt.Println()
	fmt.Println("This password has been saved to your config.db")
	fmt.Println("and will be used by remoteweather automatically.")
	fmt.Println()
}
