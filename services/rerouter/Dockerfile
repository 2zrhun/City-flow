FROM golang:1.22-alpine AS builder

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . ./
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/rerouter .

FROM alpine:3.20

RUN apk add --no-cache ca-certificates
WORKDIR /app

COPY --from=builder /out/rerouter /app/rerouter

EXPOSE 8080
ENTRYPOINT ["/app/rerouter"]
