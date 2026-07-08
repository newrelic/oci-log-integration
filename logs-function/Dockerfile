# Build stage: compile Go binary
FROM golang:1.25.12 AS build-stage

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

# Patch glibc in the base image to clear CVE-2026-5450 (flagged HIGH by the
# Oracle Linux vendor severity in Trivy). Pinned to the resolved fix version
# for reproducible builds; bump when the Trivy scan flags a newer CVE.
RUN microdnf install -y \
      glibc-2.34-272.0.1.el9_8 \
      glibc-common-2.34-272.0.1.el9_8 \
      glibc-minimal-langpack-2.34-272.0.1.el9_8 \
    && microdnf clean all

WORKDIR /function

COPY --from=build-stage /function/function /function/function

# Set permissions
RUN chmod o+r /function/function

# Entrypoint for Fn Project/OCI Functions
ENTRYPOINT ["/function/function"]