FROM golang:1.22-alpine AS builder

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . ./
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/collector .

FROM alpine:3.20

RUN apk add --no-cache ca-certificates
WORKDIR /app

COPY --from=builder /out/collector /app/collector

EXPOSE 8080
ENTRYPOINT ["/app/collector"]
