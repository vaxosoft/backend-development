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
export function lintDockerfile(_text: string): Finding[] {
  throw new Error('TODO');
}
