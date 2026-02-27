package auth

import "fmt"

// Logout removes stored OAuth credentials from ~/.claude.json and the macOS Keychain.
func Logout() error {
	removedFile, err := RemoveTokens()
	if err != nil {
		return fmt.Errorf("remove tokens from file: %w", err)
	}

	removedKeychain, _ := RemoveKeychain()

	if !removedFile && !removedKeychain {
		fmt.Println("No stored credentials found.")
		return nil
	}

	fmt.Println("Logged out successfully.")
	return nil
}
