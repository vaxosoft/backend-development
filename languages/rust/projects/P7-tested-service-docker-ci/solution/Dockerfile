# syntax=docker/dockerfile:1
#
# Build from the Cargo workspace root (languages/rust), because the crate takes its
# dependency versions from the workspace manifest and Cargo.lock:
#
#   docker build -f projects/P7-tested-service-docker-ci/starter/Dockerfile -t tasks-service .
#   docker run --rm -p 8080:8080 -e LOG_LEVEL=debug tasks-service

# ---- build stage: full toolchain, never shipped ----
FROM rust:1.99-slim-bookworm AS build
WORKDIR /src
COPY . .
# --locked: fail instead of silently updating Cargo.lock
RUN cargo build --release --locked -p p7-tested-service-docker-ci

# ---- runtime stage: only the binary ----
FROM debian:bookworm-slim
RUN useradd --system --uid 10001 --no-create-home --shell /usr/sbin/nologin app
COPY --from=build /src/target/release/p7-tested-service-docker-ci /usr/local/bin/tasks-service

ENV PORT=8080 \
    LOG_LEVEL=info \
    ENV=prod
USER app
EXPOSE 8080

# The slim image has no curl: the binary probes /healthz itself.
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["tasks-service", "healthcheck"]

ENTRYPOINT ["tasks-service"]
