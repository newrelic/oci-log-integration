# Build stage: compile Go binary
FROM golang:1.27.3 AS build-stage

WORKDIR /function

# Copy go.mod and go.sum first for dependency caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the Go binary (assuming main.go is the entrypoint)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o function main.go

# Final stage: minimal image for OCI Functions
FROM fnproject/go:1.24

# Patch known HIGH-severity CVEs in the base image's OS packages (expat, libbrotli,
# openssl, python3) -- fixes are already published upstream, fnproject's image just
# hasn't picked them up yet. See Trivy Docker Image Scan findings on this PR.
RUN microdnf update -y expat libbrotli openssl openssl-libs python3 python3-libs && microdnf clean all

WORKDIR /function

COPY --from=build-stage /function/function /function/function

# Set permissions
RUN chmod o+r /function/function

# Entrypoint for Fn Project/OCI Functions
ENTRYPOINT ["/function/function"]