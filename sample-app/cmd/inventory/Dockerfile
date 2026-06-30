FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN go build -o /out/inventory ./cmd/inventory

FROM alpine:3.20
COPY --from=build /out/inventory /usr/local/bin/inventory
EXPOSE 8083
ENTRYPOINT ["/usr/local/bin/inventory"]
