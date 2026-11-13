import assert from 'node:assert/strict';
import { describe, test } from 'node:test';
import { lintDockerfile } from './dockerfile-lint.ts';

const lines = (...text: string[]) => text.join('\n');

const GOOD = lines(
  '# syntax=docker/dockerfile:1',
  'FROM node:26-slim AS build',
  'WORKDIR /app',
  'COPY package.json package-lock.json ./',
  'RUN npm ci',
  'COPY src ./src',
  'RUN npm run build',
  '',
  'FROM node:26-slim AS prod-deps',
  'COPY package.json package-lock.json ./',
  'RUN npm ci --omit=dev',
  '',
  'FROM node:26-slim',
  'ENV NODE_ENV=production',
  'COPY --from=prod-deps /app/node_modules ./node_modules',
  'COPY --from=build /app/dist ./dist',
  'USER node',
  'CMD ["node", "dist/main.js"]',
);

describe('lintDockerfile', () => {
  test('a well-formed multi-stage Dockerfile has no findings', () => {
    assert.deepEqual(lintDockerfile(GOOD), []);
  });

  test('unpinned base images: no tag, latest, but not digests, scratch or earlier stages', () => {
    const text = lines(
      'FROM node AS a',
      'FROM node:latest AS b',
      'FROM --platform=linux/amd64 node:26-slim AS c',
      'FROM node@sha256:0123456789abcdef AS d',
      `FROM node:\${NODE_VERSION} AS e`,
      'FROM registry.local:5000/team/node AS f',
      'FROM a AS g',
      'FROM scratch',
      'USER 1000',
    );
    assert.deepEqual(
      lintDockerfile(text).filter((f) => f.rule === 'unpinned-base-image'),
      [
        { rule: 'unpinned-base-image', line: 1 },
        { rule: 'unpinned-base-image', line: 2 },
        { rule: 'unpinned-base-image', line: 6 },
      ],
    );
  });

  test('final stage must not run as root', () => {
    const noUser = lines('FROM node:26-slim', 'CMD ["node", "a.js"]');
    assert.deepEqual(lintDockerfile(noUser), [{ rule: 'runs-as-root', line: 1 }]);
    const root = lines(
      'FROM node:26-slim',
      'USER node',
      'RUN true',
      'USER root:root',
      'CMD ["node"]',
    );
    assert.deepEqual(lintDockerfile(root), [{ rule: 'runs-as-root', line: 4 }]);
    const zero = lines('FROM node:26-slim', 'USER 0', 'CMD ["node"]');
    assert.deepEqual(lintDockerfile(zero), [{ rule: 'runs-as-root', line: 2 }]);
  });

  test('USER in an earlier stage does not count for the final stage', () => {
    const text = lines(
      'FROM node:26-slim AS build',
      'USER node',
      'FROM node:26-slim',
      'CMD ["node"]',
    );
    assert.deepEqual(lintDockerfile(text), [{ rule: 'runs-as-root', line: 3 }]);
  });

  test('CMD and ENTRYPOINT must use the exec form', () => {
    const text = lines(
      'FROM node:26-slim',
      'USER node',
      'ENTRYPOINT node dist/main.js',
      'cmd npm start',
      'CMD ["node", "dist/main.js"]',
    );
    assert.deepEqual(lintDockerfile(text), [
      { rule: 'shell-form-cmd', line: 3 },
      { rule: 'shell-form-cmd', line: 4 },
    ]);
  });

  test('npm install is reported, npm ci and unrelated npm commands are not', () => {
    const text = lines(
      'FROM node:26-slim AS build',
      'RUN npm install',
      'RUN npm i --no-audit',
      'RUN npm ci && npm run build',
      'RUN npm init -y && npm run lint',
      'FROM node:26-slim',
      'USER node',
    );
    assert.deepEqual(lintDockerfile(text), [
      { rule: 'npm-install', line: 2 },
      { rule: 'npm-install', line: 3 },
    ]);
  });

  test('COPY . before the install step busts the cache', () => {
    const text = lines(
      'FROM node:26-slim AS build',
      'COPY . .',
      'RUN npm ci',
      'FROM node:26-slim',
      'COPY . ./',
      'USER node',
    );
    assert.deepEqual(lintDockerfile(text), [{ rule: 'copy-all-before-install', line: 2 }]);
  });

  test('COPY --from and COPY . after the install are fine', () => {
    const text = lines(
      'FROM node:26-slim AS build',
      'COPY package.json .',
      'RUN npm ci',
      'COPY . .',
      'FROM node:26-slim AS deps',
      'COPY --from=build . .',
      'RUN npm ci --omit=dev',
      'FROM node:26-slim',
      'USER node',
    );
    assert.deepEqual(lintDockerfile(text), []);
  });

  test('the final stage must install production dependencies only', () => {
    const text = lines(
      'FROM node:26-slim AS build',
      'RUN npm ci',
      'FROM node:26-slim',
      'RUN npm ci',
      'RUN npm ci --omit dev',
      'RUN npm ci --only=production',
      'RUN npm ci --production',
      'USER node',
    );
    assert.deepEqual(lintDockerfile(text), [{ rule: 'dev-dependencies-in-runtime', line: 4 }]);
  });

  test('line continuations, comments and instruction case', () => {
    const text = lines(
      'from node:26-slim',
      'run npm ci \\',
      '  # a comment inside the continuation',
      '  --omit=dev',
      'ENV A=1 \\',
      '    B=2',
      'cmd node \\',
      '  dist/main.js',
      'USER node',
    );
    assert.deepEqual(lintDockerfile(text), [{ rule: 'shell-form-cmd', line: 7 }]);
  });

  test('findings are sorted by line, then rule name; Windows line endings work', () => {
    const text = lines(
      'FROM node AS build',
      'COPY . .',
      'RUN npm install',
      'FROM node:26-slim',
      'RUN npm install',
      'CMD node main.js',
    ).replaceAll('\n', '\r\n');
    assert.deepEqual(lintDockerfile(text), [
      { rule: 'unpinned-base-image', line: 1 },
      { rule: 'copy-all-before-install', line: 2 },
      { rule: 'npm-install', line: 3 },
      { rule: 'runs-as-root', line: 4 },
      { rule: 'dev-dependencies-in-runtime', line: 5 },
      { rule: 'npm-install', line: 5 },
      { rule: 'shell-form-cmd', line: 6 },
    ]);
  });

  test('an empty file has no findings', () => {
    assert.deepEqual(lintDockerfile(''), []);
  });
});
