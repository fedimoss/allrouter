FROM oven/bun:1.3 AS builder

WORKDIR /build
COPY web/package.json .
COPY web/bun.lock .
RUN BUN_CONFIG_REGISTRY=https://registry.npmmirror.com bun install
COPY ./web .
COPY ./VERSION .
RUN DISABLE_ESLINT_PLUGIN='true' VITE_REACT_APP_VERSION=$(cat VERSION) bun run build

FROM golang:1.26.2-alpine3.23 AS builder2
ENV GO111MODULE=on CGO_ENABLED=0

ARG TARGETOS
ARG TARGETARCH
ENV GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64}
ENV GOEXPERIMENT=greenteagc
ENV GOPROXY=https://goproxy.cn,https://goproxy.io,direct
WORKDIR /build

ADD go.mod go.sum ./
RUN go mod tidy

COPY . .
COPY --from=builder /build/dist ./web/dist
RUN go build -ldflags "-s -w " -o allrouter

FROM debian:bookworm-slim

# 使用 sed 命令替换默认源为 阿里云 (mirrors.aliyun.com)
RUN sed -i 's/deb.debian.org/mirrors.aliyun.com/g' /etc/apt/sources.list.d/debian.sources \
    && sed -i 's/security.debian.org/mirrors.aliyun.com/g' /etc/apt/sources.list.d/debian.sources \
    && apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata libasan8 wget \
    && rm -rf /var/lib/apt/lists/* \
    && update-ca-certificates

COPY --from=builder2 /build/allrouter /
EXPOSE 3000
WORKDIR /data
ENTRYPOINT ["/allrouter"]
