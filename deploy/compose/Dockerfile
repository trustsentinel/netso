# Build the netso Phase 1 binaries (hub, agent, CLI) into a small runtime image
# with the tools the demo needs (curl for discovery/health, /bin/sh for the shell).
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN set -eux; for c in netso-hub netso-agent netso; do \
      CGO_ENABLED=0 go build -o "/out/$c" "./cmd/$c"; \
    done

FROM alpine:3.20
RUN apk add --no-cache curl
COPY --from=build /out/ /usr/local/bin/
