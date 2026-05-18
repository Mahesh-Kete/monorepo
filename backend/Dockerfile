# syntax=docker/dockerfile:1.6
#
# citadel-backend — multi-stage Go build.
#
# Pure-Go SQLite (modernc.org/sqlite) → CGO_ENABLED=0 → fully static binary →
# fits in a distroless/static runtime image.

# ---------- Stage 1: builder ----------
FROM golang:1.25-bookworm AS builder

WORKDIR /src

# Cache deps separately from sources.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ENV CGO_ENABLED=0 \
    GOOS=linux

RUN go build -trimpath -ldflags="-s -w" -o /out/citadel-backend ./cmd/backend

# ---------- Stage 2: runtime ----------
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /

COPY --from=builder /out/citadel-backend /usr/local/bin/citadel-backend

# /data is mounted from the docker-compose volume (citadel.db lives here).
# distroless/static doesn't include `mkdir`, so the compose volume must
# already exist — which it does (declared as `backend-data` in compose).
USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/citadel-backend"]
CMD ["--addr=:8080", "--db-path=/data/citadel.db"]
