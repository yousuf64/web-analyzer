# Stage 1: Build the application
FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.work go.work.sum ./
COPY api/go.mod api/go.sum ./api/
COPY analyzer/go.mod analyzer/go.sum ./analyzer/
COPY notifications/go.mod notifications/go.sum ./notifications/
COPY shared/go.mod shared/go.sum ./shared/

# Copy the source code for the shared module and the specific service
COPY shared ./shared
COPY notifications ./notifications

# Build the Go binary.
WORKDIR /app/notifications
RUN CGO_ENABLED=0 GOOS=linux go build -o /notifications/main ./cmd/main.go

# Stage 2: Create the image
FROM alpine:latest

WORKDIR /app

COPY --from=builder /notifications/main .

CMD ["./main"] 