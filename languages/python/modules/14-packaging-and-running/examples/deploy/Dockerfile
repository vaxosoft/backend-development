# syntax=docker/dockerfile:1
# Build from the examples folder:
#   docker build -f modules/14-packaging-and-running/examples/deploy/Dockerfile \
#     -t task-tracker modules/14-packaging-and-running/examples   (run from languages/python)

# ---- build stage: create a virtual environment with the pinned dependencies
FROM python:3.14-slim AS build
ENV PIP_NO_CACHE_DIR=1 PIP_DISABLE_PIP_VERSION_CHECK=1
RUN python -m venv /opt/venv
ENV PATH="/opt/venv/bin:$PATH"
WORKDIR /src
# Dependencies first: this layer is rebuilt only when requirements.txt changes.
COPY deploy/requirements.txt .
RUN pip install -r requirements.txt
COPY 04_task_service.py ./service.py

# ---- runtime stage: no compilers, no pip cache, no root
FROM python:3.14-slim AS runtime
ARG APP_VERSION=dev
ARG GIT_COMMIT=unknown
ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PATH="/opt/venv/bin:$PATH" \
    ADDR=0.0.0.0:8080 \
    APP_ENV=production \
    APP_VERSION=$APP_VERSION \
    GIT_COMMIT=$GIT_COMMIT
RUN useradd --system --uid 10001 --no-create-home app
COPY --from=build /opt/venv /opt/venv
WORKDIR /app
COPY --from=build /src/service.py .
USER 10001
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["python", "-c", "import urllib.request; urllib.request.urlopen('http://127.0.0.1:8080/healthz', timeout=2)"]
# Exec form: python is PID 1 and receives SIGTERM directly.
CMD ["python", "service.py"]
