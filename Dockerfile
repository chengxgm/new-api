FROM node:22-alpine AS web-builder

WORKDIR /build

RUN corepack enable

COPY web/package.json ./
RUN --mount=type=cache,target=~/.yarn \
    --mount=type=cache,target=~/.cache \
    yarn install --frozen-lockfile

COPY web/ ./
COPY ./VERSION ./
RUN DISABLE_ESLINT_PLUGIN=true VITE_REACT_APP_VERSION=$(cat VERSION) yarn build

FROM golang:alpine AS builder2
ENV GO111MODULE=on CGO_ENABLED=0

ARG TARGETOS
ARG TARGETARCH
ENV GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64}
ENV GOEXPERIMENT=greenteagc

WORKDIR /build

ADD go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=web-builder /build/dist ./web/dist
RUN go build -ldflags "-s -w -X 'github.com/QuantumNous/new-api/common.Version=$(cat VERSION)'" -o new-api

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata libasan8 wget \
    && rm -rf /var/lib/apt/lists/* \
    && update-ca-certificates

COPY --from=builder2 /build/new-api /
EXPOSE 3000
WORKDIR /data
ENTRYPOINT ["/new-api"]