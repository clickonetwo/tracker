FROM caddy:2.7.6-builder AS builder

RUN xcaddy build \
    --with github.com/clickonetwo/adobe_usage_tracker@v0.1.0-alpha.2

FROM caddy:2.7.6

COPY --from=builder /usr/bin/caddy /usr/bin/caddy
