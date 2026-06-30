FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o collector ./cmd/collector

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/collector .
EXPOSE 4319
CMD ["./collector"]
