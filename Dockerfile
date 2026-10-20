# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.25 AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/soil-monitor ./cmd/soil-monitor
# Distroless has no shell or mkdir, so create the data directory here and copy
# it with ownership for the nonroot user (65532).
RUN mkdir -p /out/data

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/soil-monitor /soil-monitor
COPY --from=build --chown=65532:65532 /out/data /data
ENV SOIL_HTTP_ENABLED=true \
    SOIL_HTTP_ADDR=:8080 \
    SOIL_DATABASE_PATH=/data/soil-monitor.db
VOLUME ["/data"]
EXPOSE 8080
USER 65532:65532
ENTRYPOINT ["/soil-monitor"]
