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

FROM alpine:latest

RUN apk add --no-cache tzdata
COPY --from=backend /commands/* /app/
COPY personas /personas

VOLUME /data
WORKDIR /data
ENTRYPOINT ["/app/spark"]
