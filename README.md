# HetzBot
:warning: **This project was thrown together in a few hours as a proof of concept** :warning:

Please don't use it in production :D

## What is HetzBot?
HetzBot is basically a deploy hook for Certbot that will automatically update your Hetzner Load Balancer with the new certificate.
It'll create a new certificate in Hetzner Cloud, attach it to the Load Balancer service and detach the old certificate.

## Build
```bash
docker buildx build -t hetzbot:dev .
```

## Usage
```bash
docker run -it --rm -v "$(pwd)":/mnt/cloudflare.ini --env-file .env hetzbot:dev certonly \
  --agree-tos \
  --email your-email@example.com \
  --no-eff-email \
  --expand \
  --dns-cloudflare \
  --dns-cloudflare-credentials /mnt/cloudflare.ini  \
  -d "*.example.com" -d "example.com" \
  --key-type rsa
```
