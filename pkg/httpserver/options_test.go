package httpserver

import (
	"crypto/tls"
	"net/http"
	"testing"
	"time"
)

func TestWithHTTPServer(t *testing.T) {
	s := &Server{}
	err := WithHTTPServer(nil)(s)
	if err == nil {
		t.Error("expected error for nil http.Server")
	}
	server := &http.Server{}
	err = WithHTTPServer(server)(s)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if s.Server != server {
		t.Error("server not set correctly")
	}
}

func TestWithShutdownTimeout(t *testing.T) {
	s := &Server{}
	err := WithShutdownTimeout(0)(s)
	if err == nil {
		t.Error("expected error for zero timeout")
	}
	err = WithShutdownTimeout(2 * time.Second)(s)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if s.shutdownTimeout != 2*time.Second {
		t.Error("shutdownTimeout not set correctly")
	}
}

func TestWithTLS(t *testing.T) {
	s := &Server{}
	err := WithTLS(nil)(s)
	if err == nil {
		t.Error("expected error for nil tls.Config")
	}
	ts := &tls.Config{}
	s.Server = &http.Server{}
	err = WithTLS(ts)(s)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if s.Server.TLSConfig != ts {
		t.Error("TLSConfig not set correctly")
	}
}

func TestWithAddr(t *testing.T) {
	s := &Server{}
	err := WithAddr("")(s)
	if err == nil {
		t.Error("expected error for empty address")
	}
	s.Server = &http.Server{}
	err = WithAddr("localhost:1234")(s)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if s.Server.Addr != "localhost:1234" {
		t.Error("Addr not set correctly")
	}
}
