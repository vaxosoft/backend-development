# syntax=docker/dockerfile:1

# Stage 1: compile TypeScript to JavaScript (needs devDependencies).
FROM node:26-slim AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY tsconfig.json tsconfig.build.json ./
COPY src ./src
RUN npm run build

# Stage 2: production dependencies only (native modules such as better-sqlite3 are installed here,
# on the same base image and architecture as the runtime stage).
FROM node:26-slim AS prod-deps
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --omit=dev

# Stage 3: the runtime image.
FROM node:26-slim AS runtime
ARG APP_VERSION=dev
ARG GIT_COMMIT=unknown
ENV NODE_ENV=production \
    ADDR=0.0.0.0:8080 \
    APP_VERSION=${APP_VERSION} \
    GIT_COMMIT=${GIT_COMMIT}
WORKDIR /app
COPY --from=prod-deps /app/node_modules ./node_modules
COPY --from=build /app/dist ./dist
COPY package.json ./
USER node
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s \
  CMD ["node", "-e", "fetch('http://127.0.0.1:8080/healthz').then(r => process.exit(r.ok ? 0 : 1), () => process.exit(1))"]
# Exec form: node is PID 1 and receives SIGTERM directly (npm start would swallow it).
CMD ["node", "--enable-source-maps", "dist/main.js"]
