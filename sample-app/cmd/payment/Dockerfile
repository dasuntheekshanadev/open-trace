FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN go build -o /out/payment ./cmd/payment

FROM alpine:3.20
COPY --from=build /out/payment /usr/local/bin/payment
EXPOSE 8082
ENTRYPOINT ["/usr/local/bin/payment"]
