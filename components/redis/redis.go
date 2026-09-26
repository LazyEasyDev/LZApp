package redis

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/LazyEasyDev/LZApp/config/redis_config"
	goredis "github.com/redis/go-redis/v9"
)

var keyPrefixPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]*:$`)

type Client struct {
	client goredis.UniversalClient
	prefix string
}

func New(ctx context.Context, settings *redis_config.RedisConfig) (*Client, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	options, err := clientOptions(settings)
	if err != nil {
		return nil, err
	}
	if settings.InsecureSkipVerify {
		slog.Info("Redis TLS certificate verification is disabled;")
	}
	client := goredis.NewUniversalClient(options)
	pingContext, cancel := context.WithTimeout(ctx, options.DialTimeout)
	defer cancel()
	if err := client.Ping(pingContext).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("connect to Redis: %w", err)
	}
	return &Client{client: client, prefix: settings.KeyPrefix}, nil
}

func clientOptions(settings *redis_config.RedisConfig) (*goredis.UniversalOptions, error) {
	if settings == nil {
		return nil, fmt.Errorf("Redis configuration is required")
	}
	if len(settings.Addrs) == 0 || (!settings.ClusterMode && len(settings.Addrs) != 1) {
		return nil, fmt.Errorf("Redis requires seed addresses, or exactly one address in standalone mode")
	}
	for _, address := range settings.Addrs {
		host, port, err := net.SplitHostPort(address)
		if err != nil || strings.TrimSpace(host) == "" {
			return nil, fmt.Errorf("Redis address must be host:port")
		}
		portNumber, err := strconv.Atoi(port)
		if err != nil || portNumber < 1 || portNumber > 65535 {
			return nil, fmt.Errorf("Redis port must be between 1 and 65535")
		}
	}
	if !keyPrefixPattern.MatchString(settings.KeyPrefix) {
		return nil, fmt.Errorf("Redis key prefix must start with an alphanumeric character, end with ':', and contain only letters, digits, '_', '.', ':', or '-'")
	}
	const maxTimeoutSeconds = int64((1<<63 - 1) / time.Second)
	for _, seconds := range []int{settings.DialTimeoutSeconds, settings.ReadTimeoutSeconds, settings.WriteTimeoutSeconds} {
		if seconds <= 0 || int64(seconds) > maxTimeoutSeconds {
			return nil, fmt.Errorf("Redis timeouts must be positive seconds within time.Duration range")
		}
	}
	if settings.Password != "" && settings.PasswordFile != "" {
		return nil, fmt.Errorf("configure either Redis password or password_file, not both")
	}
	password := settings.Password
	if settings.PasswordFile != "" {
		content, err := os.ReadFile(settings.PasswordFile)
		if err != nil {
			return nil, fmt.Errorf("read Redis password file: %w", err)
		}
		password = strings.TrimRight(string(content), "\r\n")
	}
	if strings.TrimSpace(settings.Username) == "" || password == "" {
		return nil, fmt.Errorf("Redis ACL username and password are required")
	}
	options := &goredis.UniversalOptions{
		Addrs:                 append([]string(nil), settings.Addrs...),
		IsClusterMode:         settings.ClusterMode,
		Username:              settings.Username,
		Password:              password,
		DialTimeout:           time.Duration(settings.DialTimeoutSeconds) * time.Second,
		ReadTimeout:           time.Duration(settings.ReadTimeoutSeconds) * time.Second,
		WriteTimeout:          time.Duration(settings.WriteTimeoutSeconds) * time.Second,
		ContextTimeoutEnabled: true,
	}
	if !settings.TLS {
		if settings.InsecureSkipVerify || settings.CACertFile != "" || settings.ServerName != "" {
			return nil, fmt.Errorf("Redis TLS options require TLS to be enabled")
		}
		return options, nil
	}
	options.TLSConfig = &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: settings.InsecureSkipVerify,
		ServerName:         settings.ServerName,
	}
	if settings.CACertFile != "" {
		certificate, err := os.ReadFile(settings.CACertFile)
		if err != nil {
			return nil, fmt.Errorf("read Redis CA certificate: %w", err)
		}
		roots, err := x509.SystemCertPool()
		if err != nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM(certificate) {
			return nil, fmt.Errorf("Redis CA file contains no valid certificates")
		}
		options.TLSConfig.RootCAs = roots
	}
	return options, nil
}

func (client *Client) Close() error {
	return client.client.Close()
}

func (client *Client) Raw() goredis.UniversalClient {
	return client.client
}

func (client *Client) Key(name string) string {
	return client.prefix + name
}

func (client *Client) Get(ctx context.Context, name string) *goredis.StringCmd {
	return client.client.Get(ctx, client.Key(name))
}

func (client *Client) Set(ctx context.Context, name string, value any, expiration time.Duration) *goredis.StatusCmd {
	return client.client.Set(ctx, client.Key(name), value, expiration)
}

func (client *Client) SetNX(ctx context.Context, name string, value any, expiration time.Duration) *goredis.BoolCmd {
	return client.client.SetNX(ctx, client.Key(name), value, expiration)
}

func (client *Client) Del(ctx context.Context, names ...string) *goredis.IntCmd {
	keys := make([]string, len(names))
	for index, name := range names {
		keys[index] = client.Key(name)
	}
	return client.client.Del(ctx, keys...)
}

func (client *Client) Exists(ctx context.Context, name string) *goredis.IntCmd {
	return client.client.Exists(ctx, client.Key(name))
}

func (client *Client) Incr(ctx context.Context, name string) *goredis.IntCmd {
	return client.client.Incr(ctx, client.Key(name))
}

func (client *Client) Expire(ctx context.Context, name string, expiration time.Duration) *goredis.BoolCmd {
	return client.client.Expire(ctx, client.Key(name), expiration)
}

func (client *Client) TTL(ctx context.Context, name string) *goredis.DurationCmd {
	return client.client.TTL(ctx, client.Key(name))
}
