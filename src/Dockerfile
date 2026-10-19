# ---- Build stage ----
FROM golang:1.27-alpine AS build
WORKDIR /app

# Copy dependency files first so this layer is cached
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build a static binary
COPY product-service.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o product-service .

# ---- Runtime stage ----
FROM alpine:3.20
WORKDIR /app
COPY --from=build /app/product-service .

EXPOSE 8080
CMD ["./product-service"]
