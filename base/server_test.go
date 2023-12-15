package base

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/myLogic207/gotils/config"
	"github.com/stretchr/testify/assert"
)

var TESTSERVER *Server
var testMsg = []byte("Hello World")

const (
	tADDRESS = "127.0.0.1"
	tPORT    = "0"
	tWORKERS = 3
)

func testHandle(ctx context.Context, conn net.Conn, args ...interface{}) error {
	println("Handling connection")
	if i, err := conn.Write(testMsg); err != nil {
		println("Error writing to connection: ", err)
		return err
	} else if i != len(testMsg) {
		println("Wrote wrong amount of bytes: ", i, " expected: ", len(testMsg))
		return errors.New("Wrote wrong amount of bytes")
	}
	println("Finished handling connection")
	return nil
}

func TestMain(m *testing.M) {
	conf := config.NewWithInitialValues(map[string]interface{}{
		"ADDRESS": tADDRESS,
		"PORT":    tPORT,
		"WORKERS": tWORKERS,
	})
	server, err := NewServer(conf, testHandle)
	if err != nil {
		panic(err)
	}
	TESTSERVER = server
	m.Run()
}

func TestServe(t *testing.T) {
	ctx := context.Background()
	if err := TESTSERVER.Listen(ctx); err != nil {
		t.Fatal(err)
	}
	go func() {
		if err := TESTSERVER.Serve(ctx); err != nil {
			panic(err)
		}
	}()

	if err := TESTSERVER.Stop(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestConnect(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	conf := config.NewWithInitialValues(map[string]interface{}{
		"ADDRESS": tADDRESS,
		"PORT":    tPORT,
		"WORKERS": 1,
	})

	server, err := NewServer(conf, testHandle)
	if err != nil {
		panic(err)
	}

	if err := server.Listen(ctx); err != nil {
		t.Fatal(err)
	}
	defer cancel()

	go func() {
		if err := server.Serve(ctx); err != nil {
			panic(err)
		}
	}()

	conn, err := net.Dial("tcp", server.GetListener().Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	<-time.After(1 * time.Millisecond)

	buff, err := io.ReadAll(conn)
	if err != nil {
		t.Fatal(err)
	}

	if assert.Equal(t, testMsg, buff) {
		t.Log("Test finished successfully")
	} else {
		t.Fatal("Test failed")
	}
}
