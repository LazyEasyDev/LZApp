package httpserver

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/LazyEasyDev/LZApp/config"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
)

type Server struct {
	api             huma.API
	server          *http.Server
	shutdownTimeout time.Duration
}

type errorWriter func(string)

func (writeError errorWriter) Write(message []byte) (int, error) {
	writeError(strings.TrimSpace(string(message)))
	return len(message), nil
}

func Init(httpConfig *config.HTTPConfig) (*Server, error) {
	if httpConfig == nil {
		return nil, fmt.Errorf("HTTP configuration is required")
	}
	if httpConfig.HTTPSPort < 1 || httpConfig.HTTPSPort > 65535 {
		return nil, fmt.Errorf("HTTPS port must be between 1 and 65535")
	}
	const maxTimeoutSeconds = int64((1<<63 - 1) / time.Second)
	for _, timeout := range []struct {
		name    string
		seconds int
	}{
		{"read_header_timeout_seconds", httpConfig.ReadHeaderTimeoutSeconds},
		{"idle_timeout_seconds", httpConfig.IdleTimeoutSeconds},
		{"read_timeout_seconds", httpConfig.ReadTimeoutSeconds},
		{"write_timeout_seconds", httpConfig.WriteTimeoutSeconds},
		{"shutdown_timeout_seconds", httpConfig.ShutdownTimeoutSeconds},
	} {
		if timeout.seconds < 0 || int64(timeout.seconds) > maxTimeoutSeconds {
			return nil, fmt.Errorf("%s must be between 0 and %d", timeout.name, maxTimeoutSeconds)
		}
	}
	if httpConfig.ShutdownTimeoutSeconds == 0 {
		return nil, fmt.Errorf("shutdown_timeout_seconds must be positive")
	}
	certificate, err := httpsCertificate(httpConfig)
	if err != nil {
		return nil, err
	}

	router := chi.NewRouter()
	huma.NewError = newError
	apiConfig := huma.DefaultConfig("LZApp API", "1.0.0")
	apiConfig.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"bearerAuth": {
			Type:   "http",
			Scheme: "bearer",
		},
	}
	apiConfig.Security = []map[string][]string{{"bearerAuth": {}}, {}}
	apiConfig.CreateHooks = nil
	apiConfig.SchemasPath = ""
	apiConfig.DocsRenderer = huma.DocsRendererScalar
	apiConfig.DocsRendererConfig = map[string]any{
		"hideClientButton":   true,
		"agent":              map[string]any{"disabled": true},
		"showDeveloperTools": "never",
		"telemetry":          false,
		"theme":              "saturn",
		"darkMode":           true,
	}
	api := humachi.New(router, apiConfig)

	server := &Server{
		api:             api,
		shutdownTimeout: time.Duration(httpConfig.ShutdownTimeoutSeconds) * time.Second,
		server: &http.Server{
			Addr:              net.JoinHostPort("", strconv.Itoa(httpConfig.HTTPSPort)),
			Handler:           router,
			ReadHeaderTimeout: time.Duration(httpConfig.ReadHeaderTimeoutSeconds) * time.Second,
			IdleTimeout:       time.Duration(httpConfig.IdleTimeoutSeconds) * time.Second,
			ReadTimeout:       time.Duration(httpConfig.ReadTimeoutSeconds) * time.Second,
			WriteTimeout:      time.Duration(httpConfig.WriteTimeoutSeconds) * time.Second,
			TLSConfig: &tls.Config{
				MinVersion:   tls.VersionTLS12,
				Certificates: []tls.Certificate{certificate},
			},
		},
	}

	server.SetErrorHandler(func(message string) {
		slog.Error("HTTPS connection error", "error", message)
	})

	return server, nil
}

func (srv *Server) API() huma.API {
	return srv.api
}

func (srv *Server) Handler() http.Handler {
	return srv.server.Handler
}

func (srv *Server) SetErrorHandler(handler func(string)) {
	if handler == nil {
		srv.server.ErrorLog = log.New(io.Discard, "", 0)
		return
	}
	srv.server.ErrorLog = log.New(errorWriter(handler), "", 0)
}

func (srv *Server) Start(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("context is required")
	}

	shutdownDone := make(chan error, 1)
	stopShutdown := context.AfterFunc(ctx, func() {
		shutdownDone <- srv.shutdown()
	})

	serveErr := serveError(srv.server.ListenAndServeTLS("", ""))
	if stopShutdown() {
		return serveErr
	}
	return errors.Join(serveErr, <-shutdownDone)
}

func (srv *Server) Close() error {
	if srv == nil || srv.server == nil {
		return nil
	}
	return srv.server.Close()
}

func (srv *Server) shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), srv.shutdownTimeout)
	defer cancel()

	if err := srv.server.Shutdown(ctx); err != nil {
		return errors.Join(fmt.Errorf("shut down HTTP server: %w", err), srv.Close())
	}
	return nil
}

func serveError(err error) error {
	if err == nil || errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return fmt.Errorf("serve HTTPS: %w", err)
}

func httpsCertificate(httpConfig *config.HTTPConfig) (tls.Certificate, error) {
	certificatePEM := strings.TrimSpace(httpConfig.HTTPSCertificate)
	keyPEM := strings.TrimSpace(httpConfig.HTTPSKey)

	switch {
	case certificatePEM == "" && keyPEM == "":
		return selfSignedCertificate()
	case certificatePEM == "" || keyPEM == "":
		return tls.Certificate{}, fmt.Errorf("HTTPS certificate and key are required")
	}

	certificate, err := tls.X509KeyPair([]byte(certificatePEM), []byte(keyPEM))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("load HTTPS certificate: %w", err)
	}
	return certificate, nil
}

func selfSignedCertificate() (tls.Certificate, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate HTTPS private key: %w", err)
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(now.UnixNano()),
		NotBefore:    now.Add(-time.Minute),
		NotAfter:     now.AddDate(1, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
	}
	certificateDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate HTTPS certificate: %w", err)
	}

	return tls.Certificate{Certificate: [][]byte{certificateDER}, PrivateKey: privateKey}, nil
}
