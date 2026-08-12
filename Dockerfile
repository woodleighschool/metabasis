# syntax=docker/dockerfile:1

# Defaults keep local and Compose builds self-contained. Renovate updates these
# alongside the matching Mise, module, and package pins.
ARG NODE_VERSION=26.7.0
ARG GO_VERSION=1.26.5

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
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS builder
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

RUN apk add --no-cache upx
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
    go build -trimpath \
    -ldflags "-s -w -X main.buildVersion=${VERSION} -X main.gitCommit=${COMMIT} -X main.buildDate=${BUILD_DATE}" \
    -o adoverseas ./cmd/server
RUN upx --best --lzma adoverseas

# ---- Runtime --------------------------------------------------------------
FROM gcr.io/distroless/static:nonroot

WORKDIR /
COPY --from=builder /workspace/adoverseas /adoverseas
EXPOSE 8080
USER 65532:65532
ENTRYPOINT ["/adoverseas"]
