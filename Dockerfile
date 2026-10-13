# syntax=docker/dockerfile:1
# Multi-stage build: static Go binary on distroless, running as non-root.
FROM golang:1.27.2-bookworm AS build
WORKDIR /src
ENV CGO_ENABLED=0 GOFLAGS=-trimpath
COPY go.mod go.sum* ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY migrations ./migrations
# Embedded content: Help Center pages (docs/help/{id,en}) and the OpenAPI spec.
COPY api ./api
COPY docs/help ./docs/help
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    go build -ldflags "-s -w \
      -X github.com/agungharyanto/sentra-dcim/internal/platform/buildinfo.Version=${VERSION} \
      -X github.com/agungharyanto/sentra-dcim/internal/platform/buildinfo.Commit=${COMMIT} \
      -X github.com/agungharyanto/sentra-dcim/internal/platform/buildinfo.BuildDate=${BUILD_DATE}" \
      -o /out/sentra ./cmd/sentra

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/sentra /sentra
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/sentra"]
CMD ["serve"]
