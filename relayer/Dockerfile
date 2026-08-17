FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY relayer/go.mod relayer/go.sum ./
RUN go mod download

COPY relayer/ ./

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-s -w' -o /app/bin/server ./cmd/server/main.go

FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/bin/server .
COPY --from=builder /app/configs ./configs

ENV GIN_MODE=release
ENV PORT=10000

EXPOSE 10000

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:10000/ || exit 1

CMD ["./server"]
