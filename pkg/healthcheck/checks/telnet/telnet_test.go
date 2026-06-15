package telnet

import (
	"context"
	"net"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestClient_Check(t *testing.T) {
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name    string
		setup   func(t *testing.T) (address string, teardown func(), ready chan struct{})
		args    args
		wantErr bool
	}{
		{
			name: "success",
			setup: func(t *testing.T) (string, func(), chan struct{}) {
				ln, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatalf("failed to start listener: %v", err)
				}
				addr := ln.Addr().String()
				stop := make(chan struct{})
				ready := make(chan struct{})
				go func() {
					t.Logf("Listener goroutine: setting deadline and signaling ready")
					defer close(stop)
					if tcpLn, ok := ln.(*net.TCPListener); ok {
						_ = tcpLn.SetDeadline(time.Now().Add(3 * time.Second))
					}
					close(ready) // signal ready to accept, right before Accept()
					t.Logf("Listener goroutine: calling Accept()")
					conn, err := ln.Accept()
					if err == nil {
						t.Logf("Listener goroutine: accepted connection")
						_ = conn.Close()
					} else {
						t.Logf("Listener goroutine: Accept() error: %v", err)
					}
					_ = ln.Close()
				}()
				return addr, func() { <-stop }, ready
			},
			args:    args{ctx: context.Background()},
			wantErr: false,
		},
		{
			name: "connection refused",
			setup: func(t *testing.T) (string, func(), chan struct{}) {
				return "127.0.0.1:65000", func() {}, make(chan struct{})
			},
			args:    args{ctx: context.Background()},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			address, teardown, ready := tt.setup(t)
			if tt.name == "success" {
				<-ready // wait for listener to be ready
			}
			t.Cleanup(teardown)
		cfg := &Config{
			Address:        address,
			RequestTimeout: metav1.Duration{Duration: 2 * time.Second}, // 2 seconds for more reliable test
		}
			t.Logf("Testing connection to %s", address)
			ctx, cancel := context.WithTimeout(tt.args.ctx, 3*time.Second)
			defer cancel()
			client, err := New(cfg)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			t.Logf("Client attempting Check()")
			err = client.Check(ctx)
			t.Logf("Client Check() returned: %v", err)
			if (err != nil) != tt.wantErr {
				t.Errorf("Check() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid tcp",
			cfg:     Config{Address: "localhost:1234", Protocol: "tcp"},
			wantErr: false,
		},
		{
			name:    "valid udp",
			cfg:     Config{Address: "localhost:1234", Protocol: "udp"},
			wantErr: false,
		},
		{
			name:    "valid default protocol",
			cfg:     Config{Address: "localhost:1234", Protocol: ""},
			wantErr: false,
		},
		{
			name:    "missing address",
			cfg:     Config{Address: "", Protocol: "tcp"},
			wantErr: true,
			errMsg:  "address is required",
		},
		{
			name:    "invalid protocol",
			cfg:     Config{Address: "localhost:1234", Protocol: "http"},
			wantErr: true,
			errMsg:  "protocol must be either 'tcp' or 'udp'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && err.Error() != tt.errMsg {
				t.Errorf("Validate() error = %v, want %v", err, tt.errMsg)
			}
		})
	}
}
