package upstream

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"strings"

	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

type TLSMode int

const (
	TLSModeAuto TLSMode = iota
	TLSModeOn
	TLSModeOff
)

func ParseTLSMode(value string) TLSMode {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "on", "1", "yes":
		return TLSModeOn
	case "false", "off", "0", "no":
		return TLSModeOff
	default:
		return TLSModeAuto
	}
}

type TLSConfig struct {
	Mode       TLSMode
	ServerName string
	CAFile     string
}

func (t TLSConfig) enabledFor(addr string) bool {
	switch t.Mode {
	case TLSModeOn:
		return true
	case TLSModeOff:
		return false
	default:
		return autoTLSEnabled(addr)
	}
}

func autoTLSEnabled(addr string) bool {
	addr = strings.TrimSpace(addr)
	if strings.HasPrefix(strings.ToLower(addr), "https://") {
		return true
	}
	_, port, err := net.SplitHostPort(normalizeGRPCAddr(addr))
	if err != nil {
		return false
	}
	return port == "443"
}

func hostFromGRPCAddr(addr string) string {
	addr = normalizeGRPCAddr(addr)
	if addr == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return strings.Trim(addr, "[]")
	}
	return strings.Trim(host, "[]")
}

func transportCredentials(addr string, tlsCfg TLSConfig) (credentials.TransportCredentials, error) {
	config, err := buildTLSConfig(addr, tlsCfg)
	if err != nil {
		return nil, err
	}
	if config == nil {
		return insecure.NewCredentials(), nil
	}
	return credentials.NewTLS(config), nil
}

func buildTLSConfig(addr string, tlsCfg TLSConfig) (*tls.Config, error) {
	if !tlsCfg.enabledFor(addr) {
		return nil, nil
	}

	serverName := strings.TrimSpace(tlsCfg.ServerName)
	if serverName == "" {
		serverName = hostFromGRPCAddr(addr)
	}
	if serverName == "" {
		return nil, fmt.Errorf("insights grpc tls: server name is required")
	}

	config := &tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: serverName,
	}
	if tlsCfg.CAFile != "" {
		pool, err := loadCertPool(tlsCfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("insights grpc tls ca file: %w", err)
		}
		config.RootCAs = pool
	}
	return config, nil
}

func loadCertPool(path string) (*x509.CertPool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM(data) {
		return nil, fmt.Errorf("no certificates found in %s", path)
	}
	return pool, nil
}
