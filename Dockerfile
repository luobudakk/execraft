FROM golang:1.26-alpine AS builder
WORKDIR /src
ENV GOPROXY=https://goproxy.cn,direct

COPY go.mod ./
COPY go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/execraft ./cmd/execraft

FROM alpine:3.20
RUN adduser -D -u 10001 execraft && apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /out/execraft /usr/local/bin/execraft
RUN mkdir -p /data && chown -R execraft:execraft /data

USER execraft
EXPOSE 8090
ENV EXECRAFT_HTTP_ADDR=:8090
ENV EXECRAFT_DATA_DIR=/data
ENV EXECRAFT_MAX_WORKERS=8
ENV EXECRAFT_QUEUE_SIZE=64
ENV EXECRAFT_SNAPSHOT_SEC=20
ENV EXECRAFT_PLUGINS=http-request

ENTRYPOINT ["execraft"]
CMD ["serve"]
