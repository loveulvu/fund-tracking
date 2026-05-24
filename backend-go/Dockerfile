
FROM golang:1.26-alpine AS builder

WORKDIR /src

RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/fund-tracking-go-api .

FROM alpine:3.22

WORKDIR /app

RUN apk add --no-cache ca-certificates \
    && adduser -D -H -u 10001 appuser

COPY --from=builder /out/fund-tracking-go-api /app/fund-tracking-go-api

USER appuser

ENV PORT=8081

EXPOSE 8081

CMD ["/app/fund-tracking-go-api"]
