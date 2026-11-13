// Reads the Dockerfile and CI workflow next to this file and summarises them (Docker is not needed).
import { readFileSync } from 'node:fs';
import { parse } from 'yaml';

const read = (name: string) => readFileSync(new URL(name, import.meta.url), 'utf8');

// Dockerfile: a stage starts at every FROM; instructions are the first word of each non-comment line.
const dockerfile = read('./Dockerfile');
const stages: { name: string; image: string; instructions: string[] }[] = [];
for (const line of dockerfile.split('\n')) {
  const from = /^FROM\s+(?<image>\S+)(?:\s+AS\s+(?<name>\S+))?/i.exec(line);
  if (from?.groups) {
    stages.push({ name: from.groups.name ?? '', image: from.groups.image ?? '', instructions: [] });
  } else if (/^[A-Z]+\s/.test(line)) {
    stages.at(-1)?.instructions.push(line.split(/\s/)[0] ?? '');
  }
}
for (const stage of stages) {
  console.log(
    `stage ${stage.name.padEnd(9)} ${stage.image.padEnd(14)} ${stage.instructions.join(' ')}`,
  );
}
const finalStage = stages.at(-1);
console.log('final stage runs as non-root:', finalStage?.instructions.includes('USER'));
console.log('CMD in exec form:', /^CMD \[/m.test(dockerfile));

// CI workflow: plain YAML, so the yaml package can load it.
interface Workflow {
  on: Record<string, unknown>;
  jobs: Record<string, { needs?: string; steps: { uses?: string; run?: string }[] }>;
}
const workflow: Workflow = parse(read('./ci.yml'));
for (const [name, job] of Object.entries(workflow.jobs)) {
  const commands = job.steps.flatMap((step) => (step.run ? [step.run] : []));
  console.log(
    `job ${name}: needs=${job.needs ?? '-'} steps=${job.steps.length} run=[${commands.join(' | ')}]`,
  );
}

// .dockerignore keeps the build context small and secrets out of the image.
const ignored = read('./.dockerignore').split('\n').filter(Boolean);
console.log(
  'dockerignore excludes node_modules and .env:',
  ['node_modules', '.env'].every((p) => ignored.includes(p)),
);
