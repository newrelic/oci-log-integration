# Use the official Go image as the base
FROM golang:1.24.4

# Set the working directory
WORKDIR /function

# Copy the Go module files and install dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the Go source code into the container
COPY main.go .

# Build the Go application
RUN go build -o function

# Set the command to run the function
CMD ["./function"]