FROM golang:alpine AS backend
RUN apk add gcc g++

WORKDIR /app
ENV CGO_ENABLED=0

COPY go.mod go.sum ./
COPY cmd ./cmd
COPY pkg ./pkg
COPY personas ./personas
COPY vendor ./vendor

RUN go build -tags=goolm -o /commands/ ./cmd/...

FROM alpine:latest AS base

RUN apk add --no-cache tzdata

VOLUME /data
WORKDIR /data

FROM base AS spark
COPY personas /personas
COPY --from=backend /commands/spark /app/
ENTRYPOINT ["/app/spark"]

FROM base AS spark-mcp-assistant
COPY --from=backend /commands/spark-mcp-assistant /app/
ENTRYPOINT ["/app/spark-mcp-assistant"]
