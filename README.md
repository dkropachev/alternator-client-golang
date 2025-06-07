# Alternator - Client-side load balancing - Go

## Glossary

- Alternator.
An DynamoDB API implemented on top of ScyllaDB backend.  
Unlike AWS DynamoDB’s single endpoint, Alternator is distributed across multiple nodes.
Could be deployed anywhere: locally, on AWS, on any cloud provider.

- Client-side load balancing.
A method where the client selects which server (node) to send requests to, 
rather than relying on a load balancing service.

- DynamoDB.
A managed NoSQL database service by AWS, typically accessed via a single regional endpoint.

- AWS Golang SDK.
The official AWS SDK for the Go programming language, used to interact with AWS services like DynamoDB.
Have two versions: [v1](https://github.com/aws/aws-sdk-go) and [v2](https://github.com/aws/aws-sdk-go-v2)

- DynamoDB/Alternator Endpoint.
The base URL a client connects to. 
In AWS DynamoDB, this is typically something like http://dynamodb.us-east-1.amazonaws.com.
In DynamoDB it is any of Alternator nodes

- Datacenter (DC).
A physical or logical grouping of racks.
On Scylla Cloud in regular setup it represents cloud provider region where nodes are deployed.

- Rack.
A logical grouping akin to an availability zone within a datacenter. 
On Scylla Cloud in regular setup it represents cloud provider availability zone where nodes are deployed.

## Introduction

This repo is a simple helper for AWS SDKs v1 and v2 to load balance load across DynamoDB nodes.
The `Helper` struct defined in `alternator_client.go` can be used to
easily change any application using [aws-sdk-go](https://github.com/aws/aws-sdk-go) from using Amazon DynamoDB
to use Alternator.

## Using the library

You create a regular `dynamodb.DynamoDB` client by one of the methods listed below and 
the rest of the application can use this dynamodb client normally
this `db` object is thread-safe and can be used from multiple threads.

This client will send requests to an Alternator nodes, instead of AWS DynamoDB.

Every request performed on patched session will pick a different live
Alternator node to send it to.
Connections to every node will be kept alive even if no requests are being sent.

### Rack and Datacenter awareness

You can configure load balancer to target particular datacenter (region) or rack (availability zone) via `WithRack` and `WithDatacenter` options, like so:
```golang
    lb, err := alb.NewHelper([]string{"x.x.x.x"}, alb.WithRack("someRack"), alb.WithDatacenter("someDc1"))
```

Additionally, you can check if alternator cluster know targeted rack/datacenter:
```golang
	if err := lb.CheckIfRackAndDatacenterSetCorrectly(); err != nil {
		return fmt.Errorf("CheckIfRackAndDatacenterSetCorrectly() unexpectedly returned an error: %v", err)
	}
```

To check if cluster support datacenter/rack feature supported you can call `CheckIfRackDatacenterFeatureIsSupported`:
```golang
    supported, err := lb.CheckIfRackDatacenterFeatureIsSupported()
	if err != nil {
		return fmt.Errorf("failed to check if rack/dc feature is supported: %v", err)
	}
	if !supported {
        return fmt.Errorf("dc/rack feature is not supporte")	
    }
```

### Spawn `dynamodb.DynamoDB`

```golang
import (
	"fmt"
    alb "alternator_loadbalancing"

    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/service/dynamodb"
)

func main() {
    lb, err := alb.NewHelper([]string{"x.x.x.x"}, alb.WithPort(9999))
    if err != nil {
        panic(fmt.Sprintf("Error creating alternator load balancer: %v", err))
    }
    ddb, err := lb.WithCredentials("whatever", "secret").NewDynamoDB()
    if err != nil {
        panic(fmt.Sprintf("Error creating dynamodb client: %v", err))
    }
    _, _ = ddb.DeleteTable(...)
}
```

## Decrypting TLS

Read wireshark wiki regarding decrypting TLS traffic: https://wiki.wireshark.org/TLS#using-the-pre-master-secret
In order to obtain pre master key secrets, you need to provide a file writer into `alb.WithKeyLogWriter`, example:

```go
	keyWriter, err := os.OpenFile("/tmp/pre-master-key.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
    if err != nil {
        panic("Error opening key writer: " + err.Error())
	}
	defer keyWriter.Close()
	lb, err := alb.NewHelper(knownNodes, alb.WithScheme("https"), alb.WithPort(httpsPort), alb.WithIgnoreServerCertificateError(true), alb.WithKeyLogWriter(keyWriter))
```

Then you need to configure your traffic analyzer to read pre master key secrets from this file.

## Example

You can find examples in `[alternator_lb_test.go](alternator_lb_test.go)`