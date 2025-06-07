package alternator_loadbalancing_v2

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"

	"github.com/dkropachev/alternator-client-golang/shared"

	smithyendpoints "github.com/aws/smithy-go/endpoints"
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
	WithOptimizeHeaders              = shared.WithOptimizeHeaders
	WithKeyLogWriter                 = shared.WithKeyLogWriter
	WithTLSSessionCache              = shared.WithTLSSessionCache
	WithMaxIdleHTTPConnections       = shared.WithMaxIdleHTTPConnections
)

type Helper struct {
	nodes *shared.AlternatorLiveNodes
	cfg   shared.Config
}

func NewHelper(initialNodes []string, options ...shared.Option) (*Helper, error) {
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

// AWSConfig produces a conf for the AWS SDK that will integrate the alternator loadbalancing with the AWS SDK.
func (lb *Helper) AWSConfig() (aws.Config, error) {
	cfg := aws.Config{
		// Region is used in the signature algorithm so prevent request sent
		// to one region to be forward by an attacker to a different region.
		// But Alternator doesn't check it. It can be anything.
		Region:       lb.cfg.AWSRegion,
		BaseEndpoint: aws.String(fmt.Sprintf("%s://%s:%d", lb.cfg.Scheme, "dynamodb.fake.alterntor.cluster.node", lb.cfg.Port)),
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
		return aws.Config{}, err
	}

	if lb.cfg.AccessKeyID != "" && lb.cfg.SecretAccessKey != "" {
		// The third credential below, the session token, is only used for
		// temporary credentials, and is not supported by Alternator anyway.
		cfg.Credentials = credentials.NewStaticCredentialsProvider(lb.cfg.AccessKeyID, lb.cfg.SecretAccessKey, "")
	}

	return cfg, nil
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

func (lb *Helper) endpointResolverV2() dynamodb.EndpointResolverV2 {
	return &EndpointResolverV2{lb: lb}
}

func (lb *Helper) NewDynamoDB() (*dynamodb.Client, error) {
	cfg, err := lb.AWSConfig()
	if err != nil {
		return nil, err
	}
	return dynamodb.NewFromConfig(cfg, dynamodb.WithEndpointResolverV2(lb.endpointResolverV2())), nil
}

type EndpointResolverV2 struct {
	lb *Helper
}

func (r *EndpointResolverV2) ResolveEndpoint(_ context.Context, _ dynamodb.EndpointParameters) (smithyendpoints.Endpoint, error) {
	return smithyendpoints.Endpoint{
		URI: r.lb.NextNode(),
	}, nil
}
