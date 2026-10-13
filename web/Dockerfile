# syntax=docker/dockerfile:1
# Sentra web UI. The host never needs Node: everything runs here (multi-arch; production is ARM64).
#   docker build --target check web/   -> supply:check, lint, typecheck, i18n:check, unit tests
#   docker build --target dist  web/   -> static assets only (/dist), consumed by deploy/caddy/Dockerfile
FROM node:22.23.3-alpine AS deps
WORKDIR /web
ENV CI=true PNPM_HOME=/pnpm COREPACK_ENABLE_DOWNLOAD_PROMPT=0
RUN corepack enable
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
RUN --mount=type=cache,id=sentra-pnpm,target=/pnpm/store pnpm install --frozen-lockfile
COPY . .

FROM deps AS check
# supply:check first: refuse any bypass of pnpm's release-age policy (ISS-031).
RUN pnpm supply:check && pnpm lint && pnpm typecheck && pnpm i18n:check && pnpm test

# Built from `check`, so nothing ships unless lint, types, catalogs and tests pass.
FROM check AS build
RUN pnpm build

FROM scratch AS dist
COPY --from=build /web/dist /dist
