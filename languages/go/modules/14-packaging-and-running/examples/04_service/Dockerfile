# syntax=docker/dockerfile:1
# Build from languages/go:
#   docker build -f modules/14-packaging-and-running/examples/04_service/Dockerfile -t tasks:dev .

FROM golang:1.27 AS build
WORKDIR /src

# Dependencies first: this layer is cached until go.mod or go.sum change.
COPY go.mod go.sum ./
RUN go mod download

COPY modules/14-packaging-and-running/examples/04_service ./modules/14-packaging-and-running/examples/04_service
ARG VERSION=dev
ARG COMMIT=none
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
    -o /out/service ./modules/14-packaging-and-running/examples/04_service

# Static binary, no shell, no package manager, non-root by default.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/service /service
USER nonroot:nonroot
ENV ADDR=0.0.0.0:8080
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s CMD ["/service", "-healthcheck"]
ENTRYPOINT ["/service"]
