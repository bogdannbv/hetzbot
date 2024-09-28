FROM golang:1.23-alpine AS build

WORKDIR /src

COPY . .

RUN apk add build-base \
    && go build \
      -o hetzbot \
      -ldflags "-linkmode external -extldflags -static"  \
      -a \
      main.go

FROM certbot/dns-cloudflare:v2.11.0

COPY --from=build /src/hetzbot /usr/local/bin/hetzbot
COPY ./certbot.ini /etc/letsencrypt/cli.ini
