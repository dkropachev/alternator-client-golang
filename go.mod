module github.com/dkropachev/alternator-client-golang

go 1.24.0

require (
	github.com/aws/aws-sdk-go v1.55.6
	github.com/dkropachev/alternator-client-golang/shared v0.0.0-00010101000000-000000000000
)

require github.com/jmespath/go-jmespath v0.4.0 // indirect

replace github.com/dkropachev/alternator-client-golang/shared => ./shared
