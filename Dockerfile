FROM golang:1.26.1-alpine3.23 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGET=api
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/cloudsentinel ./cmd/${TARGET}

# The migration image is released independently and used only by the Argo CD
# PreSync Job. It carries versioned SQL files but no application source tree.
FROM migrate/migrate:v4.18.3 AS migration
COPY --chown=65532:65532 migrations /migrations
USER 65532:65532
ENTRYPOINT ["migrate"]

FROM alpine:3.23.3 AS runtime
RUN addgroup -S -g 10001 cloudsentinel && adduser -S -D -H -u 10001 -G cloudsentinel cloudsentinel \
    && apk add --no-cache ca-certificates
COPY --from=build --chown=10001:10001 /out/cloudsentinel /usr/local/bin/cloudsentinel
USER 10001:10001
ENTRYPOINT ["/usr/local/bin/cloudsentinel"]
