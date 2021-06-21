FROM golang:1.16-buster

ENV GOPROXY="https://proxy.golang.org"
ENV GO111MODULE="on"
ENV NAT_ENV="production"

EXPOSE 8080
WORKDIR /go/src/github.com/icco/hayden

RUN wget https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb
RUN apt update && \
  apt install -y libgbm1 gconf-service libasound2 libatk1.0-0 libcairo2 libcups2 libfontconfig1 libgdk-pixbuf2.0-0 libgtk-3-0 libnspr4 libpango-1.0-0 libxss1 fonts-liberation libappindicator1 libnss3 lsb-release xdg-utils && \
  dpkg -i google-chrome-stable_current_amd64.deb && \
  apt -y install && \
  rm google-chrome-stable_current_amd64.deb
COPY . .

RUN go build -v -o /go/bin/server ./server

CMD ["/go/bin/server"]
