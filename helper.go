package alternator_client_golang

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"

	"github.com/dkropachev/alternator-client-golang/shared"
)

type Option = shared.Option

var (
	WithScheme                       = shared.WithScheme
	WithPort                         = shared.WithPort
	WithRack                         = shared.WithRack
	WithDatacenter                   = shared.WithDatacenter
	WithAWSRegion                    = shared.WithAWSRegion
	WithNodesListUpdatePeriod        = shared.WithNodesListUpdatePeriod
	WithIdleNodesListUpdatePeriod    = shared.WithIdleNodesListUpdatePeriod
	WithCredentials                  = shared.WithCredentials
	WithHTTPClient                   = shared.WithHTTPClient
	WithLocalNodesReaderHTTPClient   = shared.WithLocalNodesReaderHTTPClient
	WithClientCertificateFile        = shared.WithClientCertificateFile
	WithClientCertificate            = shared.WithClientCertificate
	WithClientCertificateSource      = shared.WithClientCertificateSource
	WithIgnoreServerCertificateError = shared.WithIgnoreServerCertificateError
	//WithOptimizeHeaders              = shared.WithOptimizeHeaders
	WithKeyLogWriter           = shared.WithKeyLogWriter
	WithTLSSessionCache        = shared.WithTLSSessionCache
	WithMaxIdleHTTPConnections = shared.WithMaxIdleHTTPConnections
)

type Helper struct {
	nodes *shared.AlternatorLiveNodes
	cfg   shared.Config
}

func NewHelper(initialNodes []string, options ...Option) (*Helper, error) {
	cfg := shared.NewConfig()
	for _, opt := range options {
		opt(cfg)
	}

	nodes, err := shared.NewAlternatorLiveNodes(initialNodes, cfg.ToALNOptions()...)
	if err != nil {
		return nil, err
	}

	return &Helper{
		nodes: nodes,
		cfg:   *cfg,
	}, nil
}

func (lb *Helper) NextNode() url.URL {
	return lb.nodes.NextNode()
}

func (lb *Helper) UpdateLiveNodes() error {
	return lb.nodes.UpdateLiveNodes()
}

func (lb *Helper) CheckIfRackAndDatacenterSetCorrectly() error {
	return lb.nodes.CheckIfRackAndDatacenterSetCorrectly()
}

func (lb *Helper) CheckIfRackDatacenterFeatureIsSupported() (bool, error) {
	return lb.nodes.CheckIfRackDatacenterFeatureIsSupported()
}

func (lb *Helper) Start() {
	lb.nodes.Start()
}

func (lb *Helper) Stop() {
	lb.nodes.Stop()
}

// AWSConfig produces a conf for the AWS SDK that will integrate the alternator loadbalancing with the AWS SDK.
func (lb *Helper) AWSConfig() (aws.Config, error) {
	cfg := aws.Config{
		Endpoint: aws.String(fmt.Sprintf("%s://%s:%d", lb.cfg.Scheme, "dynamodb.fake.alterntor.cluster.node", lb.cfg.Port)),
		// Region is used in the signature algorithm so prevent request sent
		// to one region to be forward by an attacker to a different region.
		// But Alternator doesn't check it. It can be anything.
		Region: aws.String(lb.cfg.AWSRegion),
	}

	if lb.cfg.HTTPClient != nil {
		cfg.HTTPClient = lb.cfg.HTTPClient
	} else {
		cfg.HTTPClient = &http.Client{
			Transport: shared.DefaultHTTPTransport(),
		}
	}

	err := shared.PatchHTTPClient(lb.cfg, cfg.HTTPClient)
	if err != nil {
		return cfg, err
	}

	if lb.cfg.AccessKeyID != "" && lb.cfg.SecretAccessKey != "" {
		// The third credential below, the session token, is only used for
		// temporary credentials, and is not supported by Alternator anyway.
		cfg.Credentials = credentials.NewStaticCredentials(lb.cfg.AccessKeyID, lb.cfg.SecretAccessKey, "")
	}

	cfg.HTTPClient.Transport = lb.wrapHTTPTransport(cfg.HTTPClient.Transport)
	return cfg, nil
}

func (lb *Helper) NewAWSSession() (*session.Session, error) {
	cfg, err := lb.AWSConfig()
	if err != nil {
		return nil, err
	}

	return session.NewSessionWithOptions(session.Options{
		Config: cfg,
	})
}

// WithCredentials creates clone of Helper with altered alternator credentials
func (lb *Helper) WithCredentials(accessKeyID, secretAccessKey string) *Helper {
	cfg := lb.cfg
	shared.WithCredentials(accessKeyID, secretAccessKey)(&cfg)
	return &Helper{
		nodes: lb.nodes,
		cfg:   cfg,
	}
}

// WithAWSRegion creates clone of Helper with altered AWS region
func (lb *Helper) WithAWSRegion(region string) *Helper {
	cfg := lb.cfg
	shared.WithAWSRegion(region)(&cfg)
	return &Helper{
		nodes: lb.nodes,
		cfg:   cfg,
	}
}

func (lb *Helper) NewDynamoDB() (*dynamodb.DynamoDB, error) {
	sess, err := lb.NewAWSSession()
	if err != nil {
		return nil, err
	}
	return dynamodb.New(sess), nil
}

type roundTripper struct {
	originalTransport http.RoundTripper
	lb                *Helper
}

func (rt *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	node := rt.lb.NextNode()
	req.URL = &node
	return rt.originalTransport.RoundTrip(req)
}

func (lb *Helper) wrapHTTPTransport(original http.RoundTripper) http.RoundTripper {
	return &roundTripper{
		originalTransport: original,
		lb:                lb,
	}
}
