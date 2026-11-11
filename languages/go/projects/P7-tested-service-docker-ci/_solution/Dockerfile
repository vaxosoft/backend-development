# syntax=docker/dockerfile:1

# Build context is the Go module root (languages/go), so go.mod is visible:
#   docker build -f projects/P7-tested-service-docker-ci/starter/Dockerfile -t tasks-service .
#   docker run --rm -p 8080:8080 tasks-service

FROM golang:1.27.2 AS build
WORKDIR /src
# Copy the module files first so the download layer is cached until they change.
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/server ./projects/P7-tested-service-docker-ci/starter/cmd/server

# distroless/static has no shell or package manager; the binary is fully static.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/server /server
ENV ADDR=0.0.0.0:8080 ENV=production LOG_LEVEL=info
USER 65532:65532
EXPOSE 8080
# The image has no curl, so the binary probes itself (see main.go, -healthcheck).
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/server", "-healthcheck"]
ENTRYPOINT ["/server"]
