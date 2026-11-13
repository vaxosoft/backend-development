export interface BuildInfo {
  name: string;
  version: string;
  /** Short git hash (7 characters, lowercase) or "unknown". */
  commit: string;
  /** The `engines.node` range, e.g. ">=24". */
  nodeRange: string;
}

/**
 * Builds the info shown by `GET /version` from the text of package.json and the environment.
 *
 * - throws an `Error` (with `cause` for JSON syntax errors) when the text is not a JSON object,
 *   `name` is not a non-empty string, `version` is not a semantic version (`1.2.3`, `1.2.3-rc.1`),
 *   or `engines.node` is missing.
 * - `GIT_COMMIT` (7-40 hex characters, any case) is shortened to 7 lowercase characters;
 *   a missing or malformed value becomes "unknown".
 */
export function parseBuildInfo(
  _packageJson: string,
  _env: Record<string, string | undefined>,
): BuildInfo {
  throw new Error('TODO');
}

/** `task-tracker 1.4.0 (3f9c2ab)`, or `task-tracker 1.4.0` when the commit is unknown. */
export function describeBuild(_info: BuildInfo): string {
  throw new Error('TODO');
}

/**
 * Does `nodeVersion` ("26.13.0" or "v26.13.0") satisfy `range`?
 * Only minimum ranges are supported: ">=24", ">=24.2", ">=24.2.1". Anything else throws an `Error`.
 */
export function satisfiesNode(_range: string, _nodeVersion: string): boolean {
  throw new Error('TODO');
}
