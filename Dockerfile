# Single image carrying both binaries; the Helm chart picks the entrypoint.
FROM node:22-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/server ./cmd/server && \
    CGO_ENABLED=0 go build -trimpath -o /out/collector ./cmd/collector

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/server /out/collector /usr/local/bin/
COPY --from=web /web/dist /srv/web
ENV STATIC_DIR=/srv/web
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/server"]
