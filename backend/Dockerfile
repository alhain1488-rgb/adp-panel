# Backend image — pure-Go build (modernc sqlite, no cgo) so the final image is tiny.
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/panel ./cmd/panel

FROM alpine:3.20
RUN apk add --no-cache ca-certificates wget \
    && adduser -D -u 10001 panel \
    && mkdir -p /data && chown panel:panel /data
WORKDIR /app
COPY --from=build /out/panel /app/panel
USER panel
EXPOSE 8080
ENTRYPOINT ["/app/panel"]
