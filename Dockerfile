FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY . .

WORKDIR /app/backend
RUN go build -o hotaruend .

FROM alpine:latest
WORKDIR /app

# 実行ファイルと静的ファイルのコピー
COPY --from=builder /app/backend/hotaruend /app/backend/hotaruend
COPY --from=builder /app/frontend /app/frontend

WORKDIR /app/backend
CMD ["./hotaruend"]
