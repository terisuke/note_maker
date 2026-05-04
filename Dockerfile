FROM golang:1.23-alpine AS build

WORKDIR /src

RUN apk add --no-cache build-base

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/server ./cmd/server
COPY internal ./internal
COPY static ./static

RUN CGO_ENABLED=1 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/note-maker-server ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates libgcc \
	&& mkdir -p /app /var/lib/note-maker \
	&& chown -R 65534:65534 /app /var/lib/note-maker

WORKDIR /app

COPY --from=build --chown=65534:65534 /out/note-maker-server /app/note-maker-server
COPY --from=build --chown=65534:65534 /src/static /app/static

ENV PORT=8080 \
	NOTE_MAKER_CONFIG_PATH=/var/lib/note-maker/app_config.json \
	WORKFLOW_STORE_DRIVER=sqlite \
	WORKFLOW_STORE_PATH=/var/lib/note-maker/workflow_store.db \
	MULTI_USER=0

EXPOSE 8080

USER 65534:65534

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
	CMD wget -qO- "http://127.0.0.1:${PORT}/healthz" >/dev/null || exit 1

ENTRYPOINT ["/app/note-maker-server"]
