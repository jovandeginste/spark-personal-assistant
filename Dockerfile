FROM golang:alpine AS backend

WORKDIR /app

COPY go.mod go.sum ./
COPY cmd ./cmd
COPY pkg ./pkg
COPY vendor ./vendor

RUN go build -tags=goolm -o /commands/ ./cmd/...

FROM alpine:latest

RUN apk add --no-cache tzdata
COPY --from=backend /commands/* /app/
COPY personas /personas

VOLUME /data
WORKDIR /data
ENTRYPOINT ["/app/spark"]
