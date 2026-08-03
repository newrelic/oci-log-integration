# Build stage: compile Go binary
FROM golang:1.26.5 AS build-stage

WORKDIR /function

# Copy go.mod (and go.sum once generated via `go mod tidy`) first for dep caching
COPY go.mod go.sum* ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the Go binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o function .

# Final stage: minimal image for OCI Functions
FROM fnproject/go:1.24

WORKDIR /function

COPY --from=build-stage /function/function /function/function

# Set permissions
RUN chmod o+r /function/function

# Entrypoint for Fn Project/OCI Functions
ENTRYPOINT ["/function/function"]
