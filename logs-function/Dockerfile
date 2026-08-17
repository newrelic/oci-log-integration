# Build stage: compile Go binary
FROM golang:1.26.6 AS build-stage

WORKDIR /function

# Copy go.mod and go.sum first for dependency caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the Go binary (assuming main.go is the entrypoint)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o function main.go

# Final stage: minimal image for OCI Functions
# distroless/static has no shell, package manager, or OS libraries - since the
# binary above is a static build (CGO_ENABLED=0), it has no runtime dependency
# on them either. This avoids the OS-package CVE surface that comes with
# full-distro base images (glibc, openssl, libssh, etc.).
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /function

COPY --from=build-stage /function/function /function/function

# Entrypoint for Fn Project/OCI Functions
ENTRYPOINT ["/function/function"]