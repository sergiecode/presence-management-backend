# syntax=docker/dockerfile:1

FROM golang:1.24.3-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
# Install swag for Swagger documentation generation
RUN go install github.com/swaggo/swag/cmd/swag@latest
COPY . .
# Generate Swagger documentation
RUN swag init -g cmd/main.go -o docs
RUN go build -o absti-api ./cmd/main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/absti-api .
EXPOSE 8080
CMD ["./absti-api"]
