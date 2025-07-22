FROM golang:1.23-alpine AS build-stage
WORKDIR /function
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o func

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /function
COPY --from=build-stage /function/func .
ENTRYPOINT ["./func"]