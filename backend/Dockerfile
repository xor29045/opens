# Single image, three binaries: server (API), worker (scheduled tasks), ops (CLI).
FROM golang:1.27-alpine AS build
WORKDIR /src
ENV CGO_ENABLED=0 GOFLAGS=-trimpath
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -ldflags="-s -w" -o /out/server ./cmd/server \
 && go build -ldflags="-s -w" -o /out/worker ./cmd/worker \
 && go build -ldflags="-s -w" -o /out/ops ./cmd/ops \
 && mkdir -p /keys

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/ /usr/local/bin/
# Mount point of the JWT key ring (secret volume); writable by the app user so
# that the development key can be generated on first start.
COPY --from=build --chown=nonroot:nonroot /keys /keys
USER nonroot:nonroot
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --retries=5 CMD ["/usr/local/bin/ops", "health", "check", "http://127.0.0.1:8080/healthz"]
ENTRYPOINT ["/usr/local/bin/server"]
