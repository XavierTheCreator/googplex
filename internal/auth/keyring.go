package auth

import (
	"errors"

	"github.com/zalando/go-keyring"
)

// All secrets go to the OS keychain: macOS Keychain, Windows Credential Manager,
// Linux Secret Service.
const keyringService = "googleplex"

func SaveSecret(name, value string) error { return keyring.Set(keyringService, name, value) }

// LoadSecret returns "" when the secret does not exist.
func LoadSecret(name string) (string, error) {
	v, err := keyring.Get(keyringService, name)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", nil
	}
	return v, err
}

func DeleteSecret(name string) error {
	err := keyring.Delete(keyringService, name)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}
