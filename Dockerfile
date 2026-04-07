ARG BASE_IMG=golang
ARG BASE_IMG_TAG=1-trixie

FROM --platform=$BUILDPLATFORM node:lts-alpine AS static

WORKDIR /usr/src/static
RUN apk add --no-cache libwebp libwebp-tools libavif-apps
RUN --mount=target=/usr/src/static/package.json,source=static-src/package.json --mount=target=/usr/src/static/package-lock.json,source=static-src/package-lock.json \
    npm ci

COPY --link --exclude=assets/ ./static-src/ .
ENV STYLEOUTDIR=staging/static/css/
RUN mkdir staging && npm run build
WORKDIR /usr/src/static/staging
RUN mkdir http data && chown 1000:1000 data

FROM ${BASE_IMG}:${BASE_IMG_TAG} AS builder

WORKDIR /usr/src/server

RUN --mount=target=/usr/src/server/go.mod,source=go.mod --mount=target=/usr/src/server/go.sum,source=go.sum \
    go mod download

COPY --link --exclude=static/ --exclude=static-src/  . .

RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags '-linkmode external -extldflags "-static"' \
    -o /download-count-listing cmd/downloadcountlisting/main.go

FROM ${BASE_IMG}:${BASE_IMG_TAG} AS user-creator

RUN addgroup --system --gid 1000 user && \
    adduser --system --no-create-home --disabled-password --gid 1000 --uid 1000 user

FROM scratch AS deploy
WORKDIR /srv

ARG version=develop
ARG githash=REVISION
ARG created=CREATED

COPY --link --from=user-creator /etc/passwd /etc/passwd
COPY --link --from=static /usr/src/static/staging/ .
COPY --link --from=builder --chown=root:root /download-count-listing /download-count-listing
USER 1000
EXPOSE 8080
CMD ["/download-count-listing"]
