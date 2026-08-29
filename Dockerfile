# Single image carrying both binaries; the Helm chart picks the entrypoint.
#
# Both build stages are pinned to $BUILDPLATFORM so they run natively on the
# builder instead of under QEMU: the frontend bundle is architecture-independent
# and Go cross-compiles via GOARCH. Emulating these stages for arm64 took ~30
# minutes; cross-compiling takes a few.

FROM --platform=$BUILDPLATFORM node:22-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -trimpath -o /out/server ./cmd/server && \
    CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -trimpath -o /out/collector ./cmd/collector

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/server /out/collector /usr/local/bin/
COPY --from=web /web/dist /srv/web
ENV STATIC_DIR=/srv/web
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/server"]
