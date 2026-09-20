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

const shutdownTimeout = 60 * time.Second

type Server struct {
	api    huma.API
	server *http.Server
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
	certificate, err := httpsCertificate(httpConfig)
	if err != nil {
		return nil, err
	}

	router := chi.NewRouter()
	api := humachi.New(router, huma.DefaultConfig("LZApp API", "1.0.0"))

	server := &Server{
		api: api,
		server: &http.Server{
			Addr:              net.JoinHostPort("", strconv.Itoa(httpConfig.HTTPSPort)),
			Handler:           router,
			ReadHeaderTimeout: 10 * time.Second,
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

func (myserver *Server) API() huma.API {
	return myserver.api
}

func (myserver *Server) Handler() http.Handler {
	return myserver.server.Handler
}

func (myserver *Server) SetErrorHandler(handler func(string)) {
	if handler == nil {
		myserver.server.ErrorLog = log.New(io.Discard, "", 0)
		return
	}
	myserver.server.ErrorLog = log.New(errorWriter(handler), "", 0)
}

func (myserver *Server) Start(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("context is required")
	}

	shutdownDone := make(chan error, 1)
	stopShutdown := context.AfterFunc(ctx, func() {
		shutdownDone <- myserver.shutdown()
	})

	serveErr := serveError(myserver.server.ListenAndServeTLS("", ""))
	if stopShutdown() {
		return serveErr
	}
	return errors.Join(serveErr, <-shutdownDone)
}

func (myserver *Server) Close() error {
	if myserver == nil || myserver.server == nil {
		return nil
	}
	return myserver.server.Close()
}

func (myserver *Server) shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := myserver.server.Shutdown(ctx); err != nil {
		return errors.Join(fmt.Errorf("shut down HTTP server: %w", err), myserver.Close())
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
