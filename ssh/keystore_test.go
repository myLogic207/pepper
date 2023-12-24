package ssh

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"sync"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestSSHKeystore_SetHostKey(t *testing.T) {
	// Test setting the host key
	ks := &sshKeystore{}
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Error generating key pair: %v", err)
	}
	rawPemBytes, err := ssh.MarshalPrivateKey(crypto.PrivateKey(privateKey), "")
	if err != nil {
		t.Fatalf("Error marshaling private key: %v", err)
	}
	hostKey := pem.EncodeToMemory(rawPemBytes)
	t.Log(string(hostKey))
	err = ks.SetHostKey(context.Background(), hostKey)
	if err != nil {
		t.Fatalf("Error setting host key: %v", err)
	}

	// Verify that GetHostKey returns the correct key
	retrievedKey, err := ks.GetHostKey(context.Background())
	if err != nil {
		t.Fatalf("Error getting host key: %v", err)
	}
	pubKey, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		t.Fatalf("Error creating public key: %v", err)
	}

	if !bytes.Equal(retrievedKey.PublicKey().Marshal(), pubKey.Marshal()) {
		t.Fatal("Host key does not match")
	}
}

func TestSSHKeystore_AddKnownHost_CheckKnownHost(t *testing.T) {
	ks := sshKeystore{
		userKeys: make(map[string]ssh.PublicKey),
	}
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Error generating key pair: %v", err)
	}
	pubKey, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		t.Fatalf("Error creating public key: %v", err)
	}

	hostIdentifier := "example.com"
	// Add a known host
	err = ks.AddKnownHost(context.Background(), hostIdentifier, pubKey)
	if err != nil {
		t.Fatalf("Error adding known host: %v", err)
	}

	// Check if the known host is present
	ok, err := ks.CheckKnownHost(context.Background(), hostIdentifier, pubKey)
	if err != nil {
		t.Fatalf("Error checking known host: %v", err)
	}

	if !ok {
		t.Fatal("Known host not found, but it was added")
	}

	// Check with a different key, it should return false
	otherPublicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Error generating key pair: %v", err)
	}
	otherKey, err := ssh.NewPublicKey(otherPublicKey)
	if err != nil {
		t.Fatalf("Error creating public key: %v", err)
	}

	ok, err = ks.CheckKnownHost(context.Background(), hostIdentifier, otherKey)
	if err != nil {
		t.Fatalf("Error checking known host: %v", err)
	}

	if ok {
		t.Fatal("Known host found, but it should not match the provided key")
	}
}

func TestSSHKeystore_ConcurrentAccess(t *testing.T) {
	// Test concurrent access to SetHostKey and AddKnownHost

	ks := &sshKeystore{
		userKeys: make(map[string]ssh.PublicKey),
		RWMutex:  sync.RWMutex{},
	}
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Error generating key pair: %v", err)
	}
	rawPemBytes, err := ssh.MarshalPrivateKey(crypto.PrivateKey(privateKey), "")
	if err != nil {
		t.Fatalf("Error generating key pair: %v", err)
	}
	hostKey := pem.EncodeToMemory(rawPemBytes)
	pubKey, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		t.Fatalf("Error creating public key: %v", err)
	}

	// Concurrently set the host key
	go func() {
		err := ks.SetHostKey(context.Background(), hostKey)
		if err != nil {
			t.Errorf("Error setting host key: %v", err)
		}
	}()

	// Concurrently add a known host
	go func() {
		hostIdentifier := "example.com"
		err := ks.AddKnownHost(context.Background(), hostIdentifier, pubKey)
		if err != nil {
			t.Errorf("Error adding known host: %v", err)
		}
	}()

	// Wait for goroutines to finish
}
