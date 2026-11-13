FROM node:26-slim AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY src ./src
# Emit plain JavaScript: Node then needs no type stripping and the .ts imports are rewritten to .js.
RUN npx tsc --outDir dist --rootDir src --rewriteRelativeImportExtensions \
    --target es2024 --module nodenext --moduleResolution nodenext \
    --strict --skipLibCheck --types node src/main.ts

FROM node:26-slim AS runtime
ENV NODE_ENV=production \
    ADDR=0.0.0.0:8080
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --omit=dev
COPY --from=build /app/dist ./dist
USER node
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD node -e "fetch('http://127.0.0.1:8080/healthz').then((r) => process.exit(r.ok ? 0 : 1), () => process.exit(1))"
CMD ["node", "dist/main.js"]
