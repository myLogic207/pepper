package ssh

import (
	"context"
	"database/sql"
	"errors"
	"slices"

	"golang.org/x/crypto/ssh"
)

func (s *Server) AuthLogCallback(conn ssh.ConnMetadata, method string, err error) {
	if err == nil {
		s.logger.Info(context.Background(), "Connection from '%s' using '%s'", conn.RemoteAddr().String(), method)
	} else {
		s.logger.Error(context.Background(), "Connection error from '%s' using '%s' auth: %s", conn.RemoteAddr().String(), method, err.Error())
	}
}

// PublicKeyCallback handles public key authentication.
func (s *Server) PublicKeyCallback(c ssh.ConnMetadata, pubKey ssh.PublicKey) (*ssh.Permissions, error) {
	if !slices.Contains(s.supportedKeyTypes, pubKey.Type()) {
		s.logger.Error(context.Background(), "Unsupported key type '%s'", pubKey.Type())
		return nil, ErrKeyNotSupported
	}

	if ok, err := s.keystore.CheckKnownHost(context.Background(), c.User(), pubKey); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, ErrAuthFailedReason{err}
	} else if !ok || errors.Is(err, sql.ErrNoRows) {
		return nil, ErrAuthFailed
	}

	return &ssh.Permissions{
		CriticalOptions: map[string]string{
			"pubkey-fp": ssh.FingerprintSHA256(pubKey),
		},
		Extensions: map[string]string{
			"permit-X11-forwarding":   "true",
			"permit-agent-forwarding": "true",
		},
	}, nil
}

// PasswordAuth handles password authentication.
func (s *Server) PasswordAuth(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
	return nil, ErrAuthFailedReason{errors.New("authentication method not supported")}
}

// NoAuthCallback handles scenarios where no authentication method is supported.
func (s *Server) NoAuthCallback(conn ssh.ConnMetadata) (*ssh.Permissions, error) {
	return nil, ErrAuthFailedReason{errors.New("authentication method not supported")}
}

// KeyboardInteractiveAuth handles keyboard-interactive authentication.
func (s *Server) KeyboardInteractiveAuth(conn ssh.ConnMetadata, challenge ssh.KeyboardInteractiveChallenge) (*ssh.Permissions, error) {
	return nil, ErrAuthFailedReason{errors.New("authentication method not supported")}
}
