# Build stage — static, CGO-free server binary.
FROM golang:1.27 AS builder

ENV GOPROXY="https://proxy.golang.org"
ENV CGO_ENABLED=0

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -ldflags="-s -w" -o /server ./server
RUN go build -ldflags="-s -w" -o /migrate ./cmd/migrate

# Final stage — headless-shell + server in one container (deploy contract).
FROM chromedp/headless-shell@sha256:830fa77e0602cb42e5cb2cbd9237ba27b649ad69d5fb82b54713aac8b7ab4f29

LABEL org.opencontainers.image.source=https://github.com/icco/hayden
LABEL org.opencontainers.image.description="Watches web pages and fires a webhook on a match."
LABEL org.opencontainers.image.licenses=MIT

ENV NAT_ENV="production"
ENV PORT="8080"
EXPOSE 8080

WORKDIR /app
COPY --from=builder /server .
COPY --from=builder /migrate .
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

ENTRYPOINT ["./server"]
