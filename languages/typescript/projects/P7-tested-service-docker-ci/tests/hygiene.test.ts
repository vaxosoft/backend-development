import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { test } from 'node:test';
import { parse as parseYaml } from 'yaml';

const root = join(import.meta.dirname, '..', 'starter');

// A missing file reads as '' so the starter fails on an assertion that names the file.
function readProjectFile(relativePath: string): string {
  try {
    return readFileSync(join(root, relativePath), 'utf8');
  } catch {
    return '';
  }
}

interface Instruction {
  name: string;
  args: string;
}

interface Stage {
  image: string;
  alias: string | undefined;
  instructions: Instruction[];
}

function parseDockerfile(text: string): Stage[] {
  const stages: Stage[] = [];
  const lines = text
    .replace(/\\\r?\n/g, ' ')
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter((line) => line !== '' && !line.startsWith('#'));
  for (const line of lines) {
    const [keyword = '', ...rest] = line.split(/\s+/);
    const name = keyword.toUpperCase();
    const args = rest.join(' ');
    if (name === 'FROM') {
      const match = /^(?:--\S+\s+)*(\S+)(?:\s+AS\s+(\S+))?/i.exec(args);
      stages.push({ image: match?.[1] ?? '', alias: match?.[2], instructions: [] });
    } else {
      stages.at(-1)?.instructions.push({ name, args });
    }
  }
  return stages;
}

function dockerfile(): Stage[] {
  const stages = parseDockerfile(readProjectFile('Dockerfile'));
  assert.ok(stages.length > 0, 'starter/Dockerfile is missing or has no FROM');
  return stages;
}

function finalStage(): Stage {
  const stages = dockerfile();
  return stages[stages.length - 1] as Stage;
}

function commands(stage: Stage, name: string): string[] {
  return stage.instructions.filter((i) => i.name === name).map((i) => i.args);
}

interface Workflow {
  on?: string | string[] | Record<string, unknown>;
  jobs?: Record<
    string,
    { steps?: Array<{ run?: string; uses?: string; with?: Record<string, unknown> }> }
  >;
}

function workflow(): Workflow {
  const text = readProjectFile('.github/workflows/ci.yml');
  assert.notEqual(text, '', 'starter/.github/workflows/ci.yml is missing');
  return parseYaml(text) as Workflow;
}

function ciSteps() {
  return Object.values(workflow().jobs ?? {}).flatMap((job) => job.steps ?? []);
}

test('the Dockerfile is multi-stage (at least two FROM lines)', () => {
  assert.ok(dockerfile().length >= 2);
});

test('the Dockerfile builds in an earlier stage and the final stage only copies the result', () => {
  const stages = dockerfile();
  const builder = stages[0] as Stage;
  assert.ok(
    commands(builder, 'RUN').some((run) => /\btsc\b|npm run build/.test(run)),
    'first stage must compile the code',
  );
  const last = finalStage();
  const aliases = stages.slice(0, -1).flatMap((stage, index) => [String(index), stage.alias ?? '']);
  assert.ok(
    commands(last, 'COPY').some((args) => {
      const from = /--from=(\S+)/.exec(args)?.[1];
      return from !== undefined && aliases.includes(from);
    }),
    'final stage must COPY --from=<build stage>',
  );
  assert.ok(
    !commands(last, 'RUN').some((run) => /\btsc\b/.test(run)),
    'final stage must not compile',
  );
});

test('the final stage installs production dependencies only', () => {
  const installs = commands(finalStage(), 'RUN').filter((run) => /npm (ci|install)/.test(run));
  for (const install of installs) {
    assert.match(install, /--omit=dev|--production/);
  }
});

test('the final stage runs as a non-root USER', () => {
  const users = commands(finalStage(), 'USER');
  assert.ok(users.length > 0, 'no USER instruction in the final stage');
  assert.doesNotMatch(users[users.length - 1] ?? '', /^(root|0)(:|$)/);
});

test('the final stage EXPOSEs a port', () => {
  assert.ok(commands(finalStage(), 'EXPOSE').some((args) => /^\d+/.test(args)));
});

test('the final stage has a HEALTHCHECK', () => {
  const checks = commands(finalStage(), 'HEALTHCHECK');
  assert.ok(checks.length > 0, 'no HEALTHCHECK instruction');
  assert.ok(checks.every((args) => !/^NONE/i.test(args)));
});

test('base images are pinned to a tag (never latest) and the final image is slim', () => {
  const stages = dockerfile();
  const aliases = new Set(stages.map((stage) => stage.alias));
  for (const { image } of stages.filter((stage) => !aliases.has(stage.image))) {
    assert.match(image, /:|@sha256/, `${image} has no tag`);
    assert.doesNotMatch(image, /:latest$/);
  }
  assert.match(finalStage().image, /slim|alpine|distroless/);
});

test('.dockerignore keeps node_modules, git data and env files out of the build context', () => {
  const text = readProjectFile('.dockerignore');
  assert.notEqual(text, '', 'starter/.dockerignore is missing');
  const entries = text.split(/\r?\n/).map((line) => line.trim());
  for (const expected of ['node_modules', '.git']) {
    assert.ok(entries.includes(expected), `.dockerignore must list ${expected}`);
  }
  assert.ok(
    entries.some((entry) => entry.startsWith('.env')),
    '.dockerignore must list .env files',
  );
});

test('the CI workflow runs on push and on pull_request', () => {
  const trigger = workflow().on;
  const events =
    typeof trigger === 'string'
      ? [trigger]
      : Array.isArray(trigger)
        ? trigger
        : Object.keys(trigger ?? {});
  assert.ok(events.includes('push'), 'missing push trigger');
  assert.ok(events.includes('pull_request'), 'missing pull_request trigger');
});

test('the CI workflow installs, builds, checks formatting, lints and tests', () => {
  const runs = ciSteps().flatMap((step) => (step.run ? [step.run] : []));
  const has = (pattern: RegExp) => runs.some((run) => pattern.test(run));
  assert.ok(has(/npm ci/), 'restore step (npm ci)');
  assert.ok(has(/\btsc\b|npm run build/), 'build step (tsc)');
  assert.ok(has(/biome (format|check)\b(?!.*--write)/), 'format check step without --write');
  assert.ok(has(/biome (lint|check)\b|eslint/), 'lint step');
  assert.ok(has(/node --test|npm (run )?test/), 'test step');
});

test('the CI workflow pins the Node version used by the Dockerfile', () => {
  const setup = ciSteps().find((step) => step.uses?.startsWith('actions/setup-node'));
  assert.ok(setup, 'no actions/setup-node step');
  const ciVersion = String(setup.with?.['node-version'] ?? '');
  assert.match(
    ciVersion,
    /^\d+(\.\d+){0,2}$/,
    'node-version must be a number, not "latest" or "lts/*"',
  );
  const imageMajor = /^node:(\d+)/.exec((dockerfile()[0] as Stage).image)?.[1];
  assert.equal(
    ciVersion.split('.')[0],
    imageMajor,
    'CI and Dockerfile must use the same Node major',
  );
});
