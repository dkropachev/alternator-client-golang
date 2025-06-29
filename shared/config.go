// Package shared provides basic functionality for Alternator helpers
package shared

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Config a common configuration for Alternator helper
type Config struct {
	// Port a port for alternator nodes
	Port int
	// Scheme a scheme for alternator nodes: http or https
	Scheme string
	// Rack a rack of the Alternator nodes to target
	Rack string
	// Datacenter a rack of the Alternator nodes to target
	Datacenter string
	// AWSRegion a region that will be handed over to AWS SDK to forge requests
	AWSRegion string
	// AccessKeyID from AWS credentials
	AccessKeyID string
	// SecretAccessKey from AWS credentials
	SecretAccessKey string
	// NodesListUpdatePeriod how often read list of nodes, while requests are running
	NodesListUpdatePeriod time.Duration
	// ClientCertificateSource a certificate store to supplies client certificate to the http client
	ClientCertificateSource CertSource
	// Makes it ignore server certificate errors
	IgnoreServerCertificateError bool
	// OptimizeHeaders - when true removes unnecessary http headers reducing network footprint
	OptimizeHeaders bool
	// Update node list when no requests are running
	IdleNodesListUpdatePeriod time.Duration
	// A key writer for pre master key: https://wiki.wireshark.org/TLS#using-the-pre-master-secret
	KeyLogWriter io.Writer
	// TLS session cache
	TLSSessionCache tls.ClientSessionCache
	// Maximum number of idle HTTP connections
	MaxIdleHTTPConnections int
	// Time to keep idle http connection alive
	IdleHTTPConnectionTimeout time.Duration
}

// Option a configuration option
type Option func(config *Config)

const (
	defaultPort      = 8080
	defaultScheme    = "http"
	defaultAWSRegion = "default-alb-region"
)

var defaultTLSSessionCache = tls.NewLRUClientSessionCache(256)

// NewDefaultConfig creates default `Config`
func NewDefaultConfig() *Config {
	return &Config{
		Port:                      defaultPort,
		Scheme:                    defaultScheme,
		AWSRegion:                 defaultAWSRegion,
		NodesListUpdatePeriod:     5 * time.Minute,
		IdleNodesListUpdatePeriod: 2 * time.Hour,
		TLSSessionCache:           defaultTLSSessionCache,
		MaxIdleHTTPConnections:    100,
		IdleHTTPConnectionTimeout: defaultIdleConnectionTimeout,
	}
}

// ToALNConfig converts `Config` to `ALNConfig`
func (c *Config) ToALNConfig() ALNConfig {
	cfg := NewDefaultALNConfig()
	for _, opt := range c.ToALNOptions() {
		opt(&cfg)
	}
	return cfg
}

// ToALNOptions converts `Config` to `[]ALNOption`
func (c *Config) ToALNOptions() []ALNOption {
	out := []ALNOption{
		WithALNPort(c.Port),
		WithALNScheme(c.Scheme),
		WithALNUpdatePeriod(c.NodesListUpdatePeriod),
		WithALNIgnoreServerCertificateError(c.IgnoreServerCertificateError),
		WithALNMaxIdleHTTPConnections(c.MaxIdleHTTPConnections),
		WithALNIdleHTTPConnectionTimeout(c.IdleHTTPConnectionTimeout),
	}

	if c.Rack != "" {
		out = append(out, WithALNRack(c.Rack))
	}

	if c.Datacenter != "" {
		out = append(out, WithALNDatacenter(c.Datacenter))
	}

	if c.IdleNodesListUpdatePeriod != 0 {
		out = append(out, WithALNIdleUpdatePeriod(c.IdleNodesListUpdatePeriod))
	}

	if c.ClientCertificateSource != nil {
		out = append(out, WithALNClientCertificateSource(c.ClientCertificateSource))
	}

	if c.KeyLogWriter != nil {
		out = append(out, WithALNKeyLogWriter(c.KeyLogWriter))
	}

	if c.TLSSessionCache != nil {
		out = append(out, WithALNTLSSessionCache(c.TLSSessionCache))
	}
	return out
}

// WithScheme changes schema (http/https) for both dynamodb and alternator requests
func WithScheme(scheme string) Option {
	switch scheme {
	case "http", "https":
		return func(config *Config) {
			config.Scheme = scheme
		}
	default:
		panic(fmt.Sprintf("invalid scheme: %s, supported schemas: http, https", scheme))
	}
}

// WithPort changes port for both dynamodb and alternator requests
func WithPort(port int) Option {
	return func(config *Config) {
		config.Port = port
	}
}

// WithRack makes DynamoDB client target only nodes from particular rack
func WithRack(rack string) Option {
	return func(config *Config) {
		config.Rack = rack
	}
}

// WithDatacenter makes DynamoDB client target only nodes from particular datacenter
func WithDatacenter(dc string) Option {
	return func(config *Config) {
		config.Datacenter = dc
	}
}

// WithAWSRegion inject region into DynamoDB client, this region does not play any role
// One way you can use it - to have this region in the logs, CloudWatch.
func WithAWSRegion(region string) Option {
	return func(config *Config) {
		config.AWSRegion = region
	}
}

// WithNodesListUpdatePeriod configures how often update list of nodes, while requests are running
func WithNodesListUpdatePeriod(period time.Duration) Option {
	return func(config *Config) {
		config.NodesListUpdatePeriod = period
	}
}

// WithCredentials provides credentials to DynamoDB client, which could be used by Alternator as well
func WithCredentials(accessKeyID, secretAccessKey string) Option {
	return func(config *Config) {
		config.AccessKeyID = accessKeyID
		config.SecretAccessKey = secretAccessKey
	}
}

// WithClientCertificateFile provides client certificates http clients for both DynamoDB and Alternator requests
// from files
func WithClientCertificateFile(certFile, keyFile string) Option {
	return func(config *Config) {
		config.ClientCertificateSource = NewFileCertificate(certFile, keyFile)
	}
}

// WithClientCertificate provides client certificates http clients for both DynamoDB and Alternator requests
// in a form of `tls.Certificate`
func WithClientCertificate(certificate tls.Certificate) Option {
	return func(config *Config) {
		config.ClientCertificateSource = NewCertificate(certificate)
	}
}

// WithClientCertificateSource provides client certificates http clients for both DynamoDB and Alternator requests
// in a form of custom implementation of `CertSource` interface
func WithClientCertificateSource(source CertSource) Option {
	return func(config *Config) {
		config.ClientCertificateSource = source
	}
}

// WithIgnoreServerCertificateError makes both http clients ignore tls error when value is true
func WithIgnoreServerCertificateError(value bool) Option {
	return func(config *Config) {
		config.IgnoreServerCertificateError = value
	}
}

// WithOptimizeHeaders makes DynamoDB client remove headers not used by Alternator reducing outgoing traffic
func WithOptimizeHeaders(value bool) Option {
	return func(config *Config) {
		config.OptimizeHeaders = value
	}
}

// WithIdleNodesListUpdatePeriod configures how often update list of nodes, while no requests are running
func WithIdleNodesListUpdatePeriod(period time.Duration) Option {
	return func(config *Config) {
		config.IdleNodesListUpdatePeriod = period
	}
}

// WithKeyLogWriter makes both (DynamoDB and Alternator) clients to write TLS master key into a file
// It helps to debug issues by looking at decoded HTTPS traffic between Alternator and client
func WithKeyLogWriter(writer io.Writer) Option {
	return func(config *Config) {
		config.KeyLogWriter = writer
	}
}

// WithTLSSessionCache overrides default TLS session cache
// You can use it to either provide custom TlS cache implementation or to increase/decrease it's size
func WithTLSSessionCache(cache tls.ClientSessionCache) Option {
	return func(config *Config) {
		config.TLSSessionCache = cache
	}
}

// WithMaxIdleHTTPConnections controls maximum number of http connections held by http.Transport
// Both clients configured to keep http connections to reuse them for next calls, which reduces traffic,
//
//	increases http and server efficiency and reduces latency
func WithMaxIdleHTTPConnections(value int) Option {
	return func(config *Config) {
		config.MaxIdleHTTPConnections = value
	}
}

// WithIdleHTTPConnectionTimeout controls timeout for idle http connections held by http.Transport
func WithIdleHTTPConnectionTimeout(value time.Duration) Option {
	return func(config *Config) {
		config.IdleHTTPConnectionTimeout = value
	}
}

// PatchHTTPClient takes `http.Client` instance and patches it according to `Config`
func PatchHTTPClient(config Config, client interface{}) error {
	httpClient, ok := client.(*http.Client)
	if !ok {
		return errors.New("config is not a http client")
	}
	alnConfig := config.ToALNConfig()

	if httpClient.Transport == nil {
		httpClient.Transport = DefaultHTTPTransport()
	}

	httpTransport, ok := httpClient.Transport.(*http.Transport)
	if !ok {
		return errors.New("failed to patch http transport for ignore server certificate")
	}
	PatchBasicHTTPTransport(alnConfig, httpTransport)

	if config.OptimizeHeaders {
		allowedHeaders := []string{"Host", "X-Amz-Target", "Content-Length", "Accept-Encoding"}
		if config.AccessKeyID != "" {
			allowedHeaders = append(allowedHeaders, "Authorization", "X-Amz-Date")
		}
		httpClient.Transport = NewHeaderWhiteListingTransport(httpTransport, allowedHeaders...)
	}
	return nil
}
