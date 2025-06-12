# You'll want to use this as a base but add your language servers
# Also a volume with your project in it.
FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o mcp-lsp-bridge .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/mcp-lsp-bridge .

CMD ["./mcp-lsp-bridge"]
