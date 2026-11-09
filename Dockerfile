# Go build stage
FROM golang:1.27-alpine AS go-builder
WORKDIR /build

# Module download layer is cached until go.mod/go.sum change
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY main.go ./
COPY migrations ./migrations
COPY internal ./internal
# PocketBase uses pure-Go SQLite (modernc), so no CGO/gcc is needed
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -ldflags="-s -w" -o pocketbase .

# Final stage
FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /pb
COPY --from=go-builder /build/pocketbase ./pocketbase

EXPOSE 8080
CMD ["/pb/pocketbase", "serve", "--http=0.0.0.0:8080"]
