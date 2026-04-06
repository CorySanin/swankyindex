FROM --platform=$BUILDPLATFORM node:lts-alpine AS static

WORKDIR /usr/src/static
RUN apk add --no-cache libwebp libwebp-tools libavif-apps
RUN --mount=target=/usr/src/static/package.json,source=static-src/package.json --mount=target=/usr/src/static/package-lock.json,source=static-src/package-lock.json \
    npm ci

COPY --link --exclude=assets/ ./static-src/ .
ENV STYLEOUTDIR=staging/static/css/
RUN mkdir staging && npm run build
WORKDIR /usr/src/static/staging
RUN mkdir http

FROM golang:1-trixie AS builder

WORKDIR /usr/src/server

RUN --mount=target=/usr/src/server/go.mod,source=go.mod --mount=target=/usr/src/server/go.sum,source=go.sum \
    go mod download

COPY --link --exclude=static/ --exclude=static-src/  . .

RUN CGO_ENABLED=1 GOOS=linux go build -o /download-count-listing cmd/downloadcountlisting/main.go

FROM gcr.io/distroless/base-debian13 AS deploy
WORKDIR /srv

ARG version=develop
ARG githash=REVISION
ARG created=CREATED

LABEL org.opencontainers.image.title="Download Count Listing"
LABEL org.opencontainers.image.description="HTTP directory index listing with download counts"
LABEL org.opencontainers.image.authors="Cory Sanin <corysanin@outlook.com>"
LABEL org.opencontainers.image.url="https://github.com/CorySanin/downloadcountlisting"
LABEL org.opencontainers.image.documentation="https://github.com/CorySanin/downloadcountlisting"
LABEL org.opencontainers.image.source="https://github.com/CorySanin/downloadcountlisting"
LABEL org.opencontainers.image.licenses="AGPL-3.0"
LABEL org.opencontainers.image.version="${version}"
LABEL org.opencontainers.image.revision="${githash}"
LABEL org.opencontainers.image.created="${created}"

COPY --link --from=static /usr/src/static/staging/ .
COPY --link --from=builder /download-count-listing /download-count-listing
EXPOSE 8080
CMD ["/download-count-listing"]
