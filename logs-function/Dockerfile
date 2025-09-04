# Build stage: compile Go binary
FROM golang:1.24.4 AS build-stage

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

WORKDIR /function

COPY --from=build-stage /function/function /function/function

# Set permissions
RUN chmod o+r /function/function

# Entrypoint for Fn Project/OCI Functions
ENTRYPOINT ["/function/function"]