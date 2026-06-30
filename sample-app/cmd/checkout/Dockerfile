FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN go build -o /out/checkout ./cmd/checkout

FROM alpine:3.20
COPY --from=build /out/checkout /usr/local/bin/checkout
EXPOSE 8081
ENTRYPOINT ["/usr/local/bin/checkout"]
