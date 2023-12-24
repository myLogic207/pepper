package ssh

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"net"
	"testing"

	"github.com/myLogic207/gotils/config"
	"golang.org/x/crypto/ssh"
)

const (
	tADDRESS = "127.0.0.1"
	tPORT    = "0"
)

var TESTSERVER *Server
var keystore Keystore

func TestMain(m *testing.M) {
	// Test connecting to the server
	ctx, cancel := context.WithCancel(context.Background())
	conf, err := config.WithInitialValues(ctx, map[string]interface{}{
		"ADDRESS": tADDRESS,
		"PORT":    tPORT,
		"WORKERS": 1,
	})
	if err != nil {
		panic(err)
	}

	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(fmt.Sprintf("Error generating private key: %s", err.Error()))
	}
	pemBlock, err := ssh.MarshalPrivateKey(private, "")
	if err != nil {
		panic(fmt.Sprintf("Error marshaling private key: %s", err.Error()))
	}
	privatePem := pem.EncodeToMemory(pemBlock)

	ks, err := NewKeystore(privatePem, "")
	if err != nil {
		panic(fmt.Sprintf("Error creating keystore: %s", err.Error()))
	}
	server := &Server{}
	if err := server.Listen(ctx, conf, ks); err != nil {
		panic(err)
	}

	println("Server listening on: ", server.GetAddr().String())

	go func() {
		if err := server.Serve(ctx); err != nil {
			panic(err)
		}
	}()

	keystore = ks
	TESTSERVER = server
	m.Run()

	if err := server.Stop(ctx); err != nil {
		panic(err)
	}

	cancel()
}

func TestConnect(t *testing.T) {
	clientPublic, clientPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	clientPubKey, err := ssh.NewPublicKey(clientPublic)
	if err != nil {
		t.Fatal(err)
	}

	clientPrivKey, err := ssh.NewSignerFromKey(clientPrivate)
	if err != nil {
		t.Fatal(err)
	}

	conn, err := net.Dial("tcp", TESTSERVER.GetAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	sshConf := &ssh.ClientConfig{
		User: "test",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(clientPrivKey),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	// test without host key
	// _, _, _, err = ssh.NewClientConn(conn, TESTSERVER.GetListener().Addr().String(), sshConf)
	// if err == nil {
	// 	t.Log(err)
	// }
	if err := keystore.AddKnownHost(context.Background(), "test", clientPubKey); err != nil {
		t.Fatal(err)
	}

	// test with host key
	_, _, _, err = ssh.NewClientConn(conn, TESTSERVER.GetAddr().String(), sshConf)
	if err != nil {
		t.Fatal(err)
	}

}
