# Caddy running as non-root on unprivileged port 8080.
FROM caddy:2.11.7-alpine
# Drop file capabilities so the binary runs under no-new-privileges + cap_drop ALL.
RUN apk add --no-cache libcap \
 && setcap -r /usr/bin/caddy \
 && apk del libcap \
 && mkdir -p /data/caddy /config/caddy \
 && chown -R 65532:65532 /data /config
COPY Caddyfile /etc/caddy/Caddyfile
USER 65532:65532
EXPOSE 8080
