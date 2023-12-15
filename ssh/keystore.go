package ssh

import (
	"bytes"
	"context"

	"golang.org/x/crypto/ssh"
)

type Keystore interface {
	// SetHostKey sets the private key for the host
	SetHostKey(ctx context.Context, pemBytes []byte) error
	// returns the private key for the host as signer
	GetHostKey(ctx context.Context) (key ssh.Signer, err error)
	// adds a new known host to the database
	AddKnownHost(ctx context.Context, hostIdentifier string, key ssh.PublicKey) error
	// checks if the given host is known
	CheckKnownHost(ctx context.Context, hostIdentifier string, key ssh.PublicKey) (bool, error)
	// Returns a new signed public key for a new user
	// GenerateUserKey(ctx context.Context, username string) (ssh.PublicKey, error)
}

type SSHKeystore struct {
	hostKey  ssh.Signer
	userKeys map[string]ssh.PublicKey
}

func NewSSHKeystore(privatePemBytes []byte, passphrase string) (*SSHKeystore, error) {
	signer, err := ssh.ParsePrivateKeyWithPassphrase(privatePemBytes, []byte(passphrase))
	if err != nil {
		return nil, err
	}

	return &SSHKeystore{
		hostKey:  signer,
		userKeys: make(map[string]ssh.PublicKey),
	}, nil
}

func (ks *SSHKeystore) SetHostKey(ctx context.Context, pemBytes []byte) error {
	if signer, err := ssh.ParsePrivateKey(pemBytes); err != nil {
		return err
	} else {
		ks.hostKey = signer
		return nil
	}
}

func (ks *SSHKeystore) GetHostKey(ctx context.Context) (key ssh.Signer, err error) {
	return ks.hostKey, nil
}

func (ks *SSHKeystore) AddKnownHost(ctx context.Context, hostIdentifier string, key ssh.PublicKey) error {
	ks.userKeys[hostIdentifier] = key
	return nil
}

func (ks *SSHKeystore) CheckKnownHost(ctx context.Context, hostIdentifier string, key ssh.PublicKey) (ok bool, err error) {
	if knownKey, ok := ks.userKeys[hostIdentifier]; ok {
		return ks.comparePublickeys(knownKey, key), nil
	} else {
		return false, nil
	}
}

func (ks *SSHKeystore) comparePublickeys(a, b ssh.PublicKey) bool {
	if a.Type() != b.Type() {
		return false
	}

	return bytes.Equal(a.Marshal(), b.Marshal())
}

// func (ks *SSHKeystore) GenerateUserKey(ctx context.Context, username string) (ssh.PublicKey, error) {
// 	public, privatekey, err := ed25519.GenerateKey(rand.Reader)
// 	if err != nil {
// 		return nil, err
// 	}

// 	private, err := ssh.MarshalPrivateKey(s.privateKey, "")
// 	if err != nil {
// 		return nil, err
// 	}

// }
