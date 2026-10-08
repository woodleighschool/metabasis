# syntax=docker/dockerfile:1

# Keep the container toolchain aligned with Mise. Renovate updates both.

# ---- Go build -------------------------------------------------------------
FROM --platform=$BUILDPLATFORM golang:1.27.2-alpine AS builder

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown

RUN apk add --no-cache upx
WORKDIR /workspace

COPY go.mod go.sum ./
RUN go mod download
RUN go install github.com/google/go-licenses/v2@v2.0.1

COPY cmd/ cmd/
COPY internal/ internal/

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go-licenses save ./cmd/metabasis --save_path third_party_licenses --ignore github.com/woodleighschool/metabasis --force

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -trimpath \
    -ldflags "-s -w -X main.version=${VERSION#v} -X main.commit=${COMMIT} -X main.date=${DATE}" \
    -o metabasis ./cmd/metabasis
RUN upx --best --lzma metabasis

# ---- Runtime --------------------------------------------------------------
FROM gcr.io/distroless/static:nonroot

WORKDIR /
COPY LICENSE /LICENSE
COPY --from=builder /workspace/third_party_licenses /third_party_licenses
COPY --from=builder /usr/local/go/LICENSE /third_party_licenses/go/LICENSE
COPY --from=builder /workspace/metabasis /metabasis
USER 65532:65532
ENTRYPOINT ["/metabasis"]
CMD ["run"]
