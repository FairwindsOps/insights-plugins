package upstream

import (
	"testing"
)

func TestParseTLSMode(t *testing.T) {
	tests := []struct {
		in   string
		want TLSMode
	}{
		{"auto", TLSModeAuto},
		{"", TLSModeAuto},
		{"true", TLSModeOn},
		{"on", TLSModeOn},
		{"false", TLSModeOff},
		{"off", TLSModeOff},
	}
	for _, tc := range tests {
		if got := ParseTLSMode(tc.in); got != tc.want {
			t.Fatalf("ParseTLSMode(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestAutoTLSEnabled(t *testing.T) {
	tests := []struct {
		addr string
		want bool
	}{
		{"grpc.example.com:443", true},
		{"https://grpc.example.com:443", true},
		{"grpc.example.com:4318", false},
		{"localhost:4318", false},
	}
	for _, tc := range tests {
		if got := autoTLSEnabled(tc.addr); got != tc.want {
			t.Fatalf("autoTLSEnabled(%q) = %v, want %v", tc.addr, got, tc.want)
		}
	}
}

func TestHostFromGRPCAddr(t *testing.T) {
	tests := []struct {
		addr string
		want string
	}{
		{"grpc.example.com:443", "grpc.example.com"},
		{"https://grpc.example.com:443", "grpc.example.com"},
		{"[2001:db8::1]:443", "2001:db8::1"},
	}
	for _, tc := range tests {
		if got := hostFromGRPCAddr(tc.addr); got != tc.want {
			t.Fatalf("hostFromGRPCAddr(%q) = %q, want %q", tc.addr, got, tc.want)
		}
	}
}

func TestTransportCredentialsTLS(t *testing.T) {
	creds, err := transportCredentials("grpc.example.com:443", TLSConfig{Mode: TLSModeOn})
	if err != nil {
		t.Fatalf("transportCredentials: %v", err)
	}
	info := creds.Info()
	if info.SecurityProtocol != "tls" {
		t.Fatalf("security protocol = %q, want tls", info.SecurityProtocol)
	}
}

func TestTransportCredentialsInsecure(t *testing.T) {
	creds, err := transportCredentials("localhost:4318", TLSConfig{Mode: TLSModeOff})
	if err != nil {
		t.Fatalf("transportCredentials: %v", err)
	}
	if creds.Info().SecurityProtocol != "insecure" {
		t.Fatalf("security protocol = %q, want insecure", creds.Info().SecurityProtocol)
	}
}

func TestBuildTLSConfigSetsServerName(t *testing.T) {
	cfg, err := buildTLSConfig("grpc.example.com:443", TLSConfig{Mode: TLSModeOn})
	if err != nil {
		t.Fatalf("buildTLSConfig: %v", err)
	}
	if cfg.ServerName != "grpc.example.com" {
		t.Fatalf("server name = %q, want grpc.example.com", cfg.ServerName)
	}
}

func TestBuildTLSConfigCustomServerName(t *testing.T) {
	cfg, err := buildTLSConfig("10.0.0.1:443", TLSConfig{
		Mode:       TLSModeOn,
		ServerName: "grpc.example.com",
	})
	if err != nil {
		t.Fatalf("buildTLSConfig: %v", err)
	}
	if cfg.ServerName != "grpc.example.com" {
		t.Fatalf("server name = %q, want grpc.example.com", cfg.ServerName)
	}
}

func TestBuildTLSConfigDisabled(t *testing.T) {
	cfg, err := buildTLSConfig("localhost:4318", TLSConfig{Mode: TLSModeOff})
	if err != nil {
		t.Fatalf("buildTLSConfig: %v", err)
	}
	if cfg != nil {
		t.Fatal("expected nil tls config when disabled")
	}
}
