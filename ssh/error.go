package ssh

import (
	"errors"
	"fmt"
)

var (
	// ErrKeyNotSupported indicates that the key type is not supported.
	ErrKeyNotSupported = errors.New("key type not supported")

	// ErrAuthFailed indicates a generic authentication failure.
	ErrAuthFailed = errors.New("authentication failed")

	// ErrSSHConfig indicates an error loading the SSH config.
	ErrSSHConfig = errors.New("error loading ssh config")

	// ErrNoPrivateKey indicates that no private key was provided.
	ErrNoPrivateKey = errors.New("no private key provided")
)

type ErrSSHConfigReason struct {
	reason error
}

func (e ErrSSHConfigReason) Error() string {
	return fmt.Sprintf("error loading ssh config: %s", e.reason.Error())
}

func (e ErrSSHConfigReason) Unwrap() error {
	return ErrSSHConfig
}

// ErrAuthFailedReason provides additional context for authentication failure.
type ErrAuthFailedReason struct {
	reason error
}

// Error returns the formatted error message.
func (e ErrAuthFailedReason) Error() string {
	return fmt.Sprintf("authentication failed: %s", e.reason.Error())
}

// Unwrap returns the underlying error.
func (e ErrAuthFailedReason) Unwrap() error {
	return ErrAuthFailed
}
