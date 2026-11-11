# Multi-stage build for a standalone Rust service (Cargo.toml, Cargo.lock and src/ at the repo root).
# Not built in this repository: Docker is not available on the authoring machine.

# ---- build stage: full toolchain, never shipped -------------------------------------------
FROM rust:1.99-slim-bookworm AS build
WORKDIR /app

# 1) Dependencies only. This layer is rebuilt only when Cargo.toml or Cargo.lock change.
COPY Cargo.toml Cargo.lock ./
RUN mkdir src && echo 'fn main() {}' > src/main.rs \
    && cargo build --release --locked \
    && rm -rf src

# 2) The real sources. `touch` makes cargo notice they are newer than the dummy main.
COPY src ./src
RUN touch src/main.rs && cargo build --release --locked

# ---- runtime stage: no compiler, no shell, no package manager ----------------------------
FROM gcr.io/distroless/cc-debian12:nonroot
COPY --from=build /app/target/release/tasks-service /usr/local/bin/tasks-service

ENV ADDR=0.0.0.0:8080
EXPOSE 8080
USER nonroot:nonroot

# Distroless has no curl: the binary probes itself (see `--healthcheck` in the tracker_service example).
HEALTHCHECK --interval=15s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/usr/local/bin/tasks-service", "--healthcheck"]

ENTRYPOINT ["/usr/local/bin/tasks-service"]
