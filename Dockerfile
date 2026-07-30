# syntax=docker/dockerfile:1

# ARGs used in a FROM must live in the global scope (before the first FROM).
# Both versions are supplied by the release workflow from Mise.
ARG NODE_VERSION
ARG GO_VERSION

# ---- Web build ------------------------------------------------------------
# Build the frontend bundle so the Go stage can embed it. The runtime image
# does not include Node.
FROM --platform=$BUILDPLATFORM node:${NODE_VERSION}-alpine AS web
WORKDIR /workspace/web

# Install dependencies against the lockfile first for layer caching.
COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
RUN npm install --global "$(node --print 'require("./package.json").packageManager')"
RUN pnpm install --frozen-lockfile

COPY web/ ./
RUN pnpm build

# ---- Go build -------------------------------------------------------------
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace

# Cache module downloads before copying source.
COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ cmd/
COPY internal/ internal/
COPY web/ web/

# Overlay the freshly built frontend bundle so go:embed uses the real assets.
COPY --from=web /workspace/web/dist web/dist

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags "-s -w" -o adoverseas ./cmd/server

# ---- Runtime --------------------------------------------------------------
FROM gcr.io/distroless/static:nonroot

WORKDIR /
COPY --from=builder /workspace/adoverseas /adoverseas
EXPOSE 8080
USER 65532:65532
ENTRYPOINT ["/adoverseas"]
