# Caddy running as non-root. Ports 80/443 are bound via the container sysctl
# net.ipv4.ip_unprivileged_port_start=0 (see docker-compose.yml), so no capability is needed.
FROM caddy:2.11.7-alpine
# Drop file capabilities so the binary runs under no-new-privileges + cap_drop ALL.
RUN apk add --no-cache libcap \
 && setcap -r /usr/bin/caddy \
 && apk del libcap \
 && mkdir -p /data/caddy /config/caddy \
 && chown -R 65532:65532 /data /config
COPY Caddyfile /etc/caddy/Caddyfile
# Static web UI built by web/Dockerfile (target dist); docker-compose.yml supplies it as the "web" build context.
COPY --from=web /dist /srv/web
USER 65532:65532
EXPOSE 80 443 8081
