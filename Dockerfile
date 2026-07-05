# Build stage — static, CGO-free server binary.
FROM golang:1.26 AS builder

ENV GOPROXY="https://proxy.golang.org"
ENV CGO_ENABLED=0

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -ldflags="-s -w" -o /server ./server
RUN go build -ldflags="-s -w" -o /migrate ./cmd/migrate

# Final stage — headless-shell + server in one container (deploy contract).
FROM chromedp/headless-shell@sha256:6b48adef158c57ef401977e1d18e4100c605b0407162e3d12f7c80b1b9bfdaaa

LABEL org.opencontainers.image.source=https://github.com/icco/hayden
LABEL org.opencontainers.image.description="Watches web pages and fires a webhook on a match."
LABEL org.opencontainers.image.licenses=MIT

ENV NAT_ENV="production"
ENV PORT="8080"
EXPOSE 8080

WORKDIR /app
COPY --from=builder /server .
COPY --from=builder /migrate .
COPY start.sh .

ENTRYPOINT ["./start.sh"]
