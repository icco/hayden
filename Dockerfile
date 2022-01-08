FROM golang:1.17

ENV GOPROXY="https://proxy.golang.org"
ENV GO111MODULE="on"
ENV NAT_ENV="production"

WORKDIR /go/src/github.com/icco/hayden

COPY . .

RUN go build -v -o /go/bin/server ./server

FROM chromedp/headless-shell:latest

ENV NAT_ENV "production"
ENV PORT 8080
EXPOSE $PORT
COPY --from=0 /go/bin/server .
COPY ./start.sh .

CMD ["./start.sh"]
