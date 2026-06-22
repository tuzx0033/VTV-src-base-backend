# ── build stage ───────────────────────────────────────────────────────────────
FROM golang:1.24-alpine AS build
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
ARG TARGET=api                                   # api | worker | migrate | seed-admin
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X main.Version=${VERSION}" -o /out/app ./cmd/${TARGET}

# ── runtime stage ─────────────────────────────────────────────────────────────
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 app
USER app
WORKDIR /app
COPY --from=build /out/app /app/app
COPY config ./config
COPY migrations ./migrations
ENV TZ=Asia/Ho_Chi_Minh
EXPOSE 8080
ENTRYPOINT ["/app/app"]
CMD ["-env", "prod"]
