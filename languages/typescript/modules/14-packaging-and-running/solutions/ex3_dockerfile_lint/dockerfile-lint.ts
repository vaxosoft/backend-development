export type RuleId =
  | 'unpinned-base-image'
  | 'runs-as-root'
  | 'shell-form-cmd'
  | 'npm-install'
  | 'copy-all-before-install'
  | 'dev-dependencies-in-runtime';

export interface Finding {
  rule: RuleId;
  /** 1-based number of the first physical line of the instruction. */
  line: number;
}

interface Instruction {
  name: string;
  args: string;
  line: number;
  stage: number;
}

const NPM_INSTALL = /\bnpm\s+(?:install|i)\b/;
const NPM_ANY_INSTALL = /\bnpm\s+(?:ci|install|i)\b/;
const PRODUCTION_ONLY = /--omit[= ]dev\b|--only[= ]production\b|--production\b/;

function parse(text: string): Instruction[] {
  const instructions: Instruction[] = [];
  let stage = -1;
  let pending: { line: number; text: string } | null = null;

  const flush = () => {
    if (!pending) return;
    const [, name = '', args = ''] = /^(\S+)\s*(.*)$/s.exec(pending.text.trim()) ?? [];
    if (name.toUpperCase() === 'FROM') stage += 1;
    instructions.push({ name: name.toUpperCase(), args, line: pending.line, stage });
    pending = null;
  };

  text.split(/\r?\n/).forEach((raw, index) => {
    const line = raw.trim();
    if (line.startsWith('#') || (line === '' && pending === null)) return;
    if (line === '') return;
    pending ??= { line: index + 1, text: '' };
    const continues = line.endsWith('\\');
    pending.text += `${continues ? line.slice(0, -1) : line} `;
    if (!continues) flush();
  });
  flush();
  return instructions;
}

function words(args: string): string[] {
  return args.split(/\s+/).filter(Boolean);
}

function isUnpinned(image: string): boolean {
  if (image.includes('@')) return false;
  const lastSegment = image.slice(image.lastIndexOf('/') + 1);
  const colon = lastSegment.indexOf(':');
  return colon === -1 || lastSegment.slice(colon + 1) === 'latest';
}

/**
 * Checks the text of a Dockerfile and returns the findings sorted by line, then by rule name.
 *
 * Parsing: comment lines and blank lines are ignored (also inside a `\` continuation); a line ending in
 * `\` continues on the next line; the instruction name is case-insensitive.
 *
 * Rules (a "stage" starts at each `FROM`; the *final stage* is the last one):
 * - `unpinned-base-image`: `FROM` image without a tag/digest or with tag `latest`. Ignore `scratch`,
 *   names of earlier stages (`FROM build`) and `--platform=...` flags. A tag that is a variable
 *   (`node:${NODE_VERSION}`) counts as pinned. Line of the `FROM`.
 * - `runs-as-root`: the final stage has no `USER`, or its last `USER` is `root` or `0` (user part
 *   before a `:`). Line of the last `USER`, or of the final `FROM` when there is none.
 * - `shell-form-cmd`: `CMD` or `ENTRYPOINT` whose argument is not a JSON array (does not start with `[`).
 * - `npm-install`: a `RUN` containing `npm install` or `npm i` as a command (use `npm ci`).
 * - `copy-all-before-install`: `COPY . ...` (source `.`, no `--from`) followed later in the same stage by a
 *   `RUN` with `npm ci`/`npm install`: it busts the dependency layer cache. Line of the `COPY`.
 * - `dev-dependencies-in-runtime`: in the final stage, a `RUN` with `npm ci`/`npm install` without
 *   `--omit=dev`, `--omit dev`, `--only=production` or `--production`.
 */
export function lintDockerfile(text: string): Finding[] {
  const instructions = parse(text);
  const findings: Finding[] = [];
  const add = (rule: RuleId, line: number) => findings.push({ rule, line });
  const finalStage = instructions.reduce((max, i) => Math.max(max, i.stage), -1);
  const stageNames = new Set<string>();

  for (const [index, instruction] of instructions.entries()) {
    const { name, args, line, stage } = instruction;
    if (name === 'FROM') {
      const [image = '', as, alias] = words(args).filter((word) => !word.startsWith('--'));
      if (image !== 'scratch' && !stageNames.has(image.toLowerCase()) && isUnpinned(image)) {
        add('unpinned-base-image', line);
      }
      if (as?.toUpperCase() === 'AS' && alias) stageNames.add(alias.toLowerCase());
    } else if (name === 'CMD' || name === 'ENTRYPOINT') {
      if (!args.startsWith('[')) add('shell-form-cmd', line);
    } else if (name === 'RUN') {
      if (NPM_INSTALL.test(args)) add('npm-install', line);
      if (stage === finalStage && NPM_ANY_INSTALL.test(args) && !PRODUCTION_ONLY.test(args)) {
        add('dev-dependencies-in-runtime', line);
      }
    } else if (name === 'COPY') {
      const operands = words(args);
      const copiesAll = operands.every((word) => !word.startsWith('--from'));
      const source = operands.find((word) => !word.startsWith('--'));
      if (copiesAll && (source === '.' || source === './')) {
        const installsLater = instructions
          .slice(index + 1)
          .some(
            (next) =>
              next.stage === stage && next.name === 'RUN' && NPM_ANY_INSTALL.test(next.args),
          );
        if (installsLater) add('copy-all-before-install', line);
      }
    }
  }

  const finalInstructions = instructions.filter((i) => i.stage === finalStage);
  const finalFrom = finalInstructions.find((i) => i.name === 'FROM');
  if (finalFrom) {
    const lastUser = finalInstructions.filter((i) => i.name === 'USER').at(-1);
    const user = lastUser?.args.split(':')[0]?.trim();
    if (!lastUser) add('runs-as-root', finalFrom.line);
    else if (user === 'root' || user === '0') add('runs-as-root', lastUser.line);
  }

  return findings.sort((a, b) => a.line - b.line || a.rule.localeCompare(b.rule));
}
