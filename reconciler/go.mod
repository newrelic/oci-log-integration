module github.com/newrelic/oci-log-integration/reconciler

go 1.25.9

require (
	github.com/fnproject/fdk-go v0.1.15
	github.com/oracle/oci-go-sdk/v65 v65.121.0
)

// POC: run `go mod tidy` in an environment with network + Go toolchain to
// populate go.sum before building.
