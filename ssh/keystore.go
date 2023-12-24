package ssh

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"sync"

	"golang.org/x/crypto/ssh"
)

var (
	ErrNoHostKey = errors.New("no host key set")
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

type sshKeystore struct {
	sync.RWMutex
	hostKey  ssh.Signer
	userKeys map[string]ssh.PublicKey
}

func NewKeystore(privatePemBytes []byte, passphrase string) (Keystore, error) {
	if len(privatePemBytes) == 0 {
		return nil, ErrNoPrivateKey
	}
	var signer ssh.Signer
	var err error
	if len(passphrase) == 0 {
		signer, err = ssh.ParsePrivateKey(privatePemBytes)
	} else {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(privatePemBytes, []byte(passphrase))
	}
	if err != nil {
		return nil, err
	}

	return &sshKeystore{
		hostKey:  signer,
		userKeys: make(map[string]ssh.PublicKey),
	}, nil
}

func (ks *sshKeystore) SetHostKey(ctx context.Context, pemBytes []byte) error {
	if signer, err := ssh.ParsePrivateKey(pemBytes); err != nil {
		return err
	} else {
		ks.Lock()
		ks.hostKey = signer
		ks.Unlock()
		return nil
	}
}

func (ks *sshKeystore) GetHostKey(ctx context.Context) (key ssh.Signer, err error) {
	ks.RLock()
	if ks.hostKey == nil {
		err = ErrNoHostKey
	} else {
		key = ks.hostKey
	}
	ks.RUnlock()
	return
}

func (ks *sshKeystore) AddKnownHost(ctx context.Context, hostIdentifier string, key ssh.PublicKey) error {
	ks.Lock()
	ks.userKeys[hostIdentifier] = key
	ks.Unlock()
	return nil
}

func (ks *sshKeystore) CheckKnownHost(ctx context.Context, hostIdentifier string, key ssh.PublicKey) (ok bool, err error) {
	ks.RLock()
	knownKey, ok := ks.userKeys[hostIdentifier]
	ks.RUnlock()
	if !ok {
		return false, nil
	}
	return ks.comparePublickeys(knownKey, key), nil
}

func (ks *sshKeystore) comparePublickeys(a, b ssh.PublicKey) bool {
	if a.Type() != b.Type() {
		return false
	}

	return bytes.Equal(a.Marshal(), b.Marshal())
}

func GenerateKey() (crypto.PrivateKey, error) {
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return private, nil
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
