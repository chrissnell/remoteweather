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

	// ANSI color codes for terminal output
	ColorReset   = "\033[0m"
	ColorRed     = "\033[31m"
	ColorGreen   = "\033[32m"
	ColorYellow  = "\033[33m"
	ColorBlue    = "\033[34m"
	ColorMagenta = "\033[35m"
	ColorCyan    = "\033[36m"
	ColorWhite   = "\033[37m"
	ColorBrightRed     = "\033[91m"
	ColorBrightGreen   = "\033[92m"
	ColorBrightYellow  = "\033[93m"
	ColorBrightBlue    = "\033[94m"
	ColorBrightMagenta = "\033[95m"
	ColorBrightCyan    = "\033[96m"
	ColorBold          = "\033[1m"
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
	fmt.Printf("%s%s🔐 Generated Password%s\n", ColorBold, ColorBrightYellow, ColorReset)
	fmt.Printf("%s╔════════════════════════════════════════════════╗%s\n", ColorBrightYellow, ColorReset)
	fmt.Printf("%s║  ⚠️  SAVE THIS PASSWORD - IT WON'T BE SHOWN AGAIN  ║%s\n", ColorBrightYellow, ColorReset)
	fmt.Printf("%s╚════════════════════════════════════════════════╝%s\n", ColorBrightYellow, ColorReset)
	fmt.Println()
	fmt.Printf("  %sPassword: %s%s%s%s\n", ColorBold, ColorBrightCyan, password, ColorReset, ColorReset)
	fmt.Println()
	fmt.Println("This password has been saved to your config.db")
	fmt.Println("and will be used by remoteweather automatically.")
	fmt.Println()
}

// DisplayBothPasswords prints both the postgres and remoteweather passwords
func DisplayBothPasswords(postgresPassword, remoteweatherPassword string) {
	fmt.Println()
	fmt.Printf("%s%s🔐 Generated Passwords%s\n", ColorBold, ColorBrightYellow, ColorReset)
	fmt.Printf("%s╔════════════════════════════════════════════════╗%s\n", ColorBrightYellow, ColorReset)
	fmt.Printf("%s║  ⚠️  SAVE THESE PASSWORDS - THEY WON'T BE SHOWN AGAIN  ║%s\n", ColorBrightYellow, ColorReset)
	fmt.Printf("%s╚════════════════════════════════════════════════╝%s\n", ColorBrightYellow, ColorReset)
	fmt.Println()
	fmt.Printf("  %sPostgreSQL 'postgres' user password:%s\n", ColorBold, ColorReset)
	fmt.Printf("  %s%s%s\n", ColorBrightCyan, postgresPassword, ColorReset)
	fmt.Println()
	fmt.Printf("  %sRemoteweather database user password:%s\n", ColorBold, ColorReset)
	fmt.Printf("  %s%s%s\n", ColorBrightCyan, remoteweatherPassword, ColorReset)
	fmt.Println()
	fmt.Println("The remoteweather password has been saved to your config.db")
	fmt.Println("and will be used by remoteweather automatically.")
	fmt.Println()
	fmt.Println("The postgres password is for administrative access only.")
	fmt.Println("You typically won't need it for normal operations.")
	fmt.Println()
}
