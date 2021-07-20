FROM golang:1.16-buster

ENV GOPROXY="https://proxy.golang.org"
ENV GO111MODULE="on"
ENV NAT_ENV="production"

WORKDIR /go/src/github.com/icco/hayden

COPY . .

RUN go build -v -o /go/bin/server ./server

FROM chromedp/headless-shell:latest

EXPOSE 8080
ENV NAT_ENV="production"
COPY --from=0 /go/bin/server .

CMD ["./server"]
