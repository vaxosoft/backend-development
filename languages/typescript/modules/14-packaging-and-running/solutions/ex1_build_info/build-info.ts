export interface BuildInfo {
  name: string;
  version: string;
  /** Short git hash (7 characters, lowercase) or "unknown". */
  commit: string;
  /** The `engines.node` range, e.g. ">=24". */
  nodeRange: string;
}

const SEMVER = /^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/;
const COMMIT = /^[0-9a-f]{7,40}$/i;

function fail(message: string, cause?: unknown): never {
  throw new Error(`invalid package.json: ${message}`, { cause });
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
  packageJson: string,
  env: Record<string, string | undefined>,
): BuildInfo {
  let data: unknown;
  try {
    data = JSON.parse(packageJson);
  } catch (error) {
    fail('not valid JSON', error);
  }
  if (typeof data !== 'object' || data === null || Array.isArray(data)) fail('not an object');
  const { name, version, engines } = data as Record<string, unknown>;
  if (typeof name !== 'string' || name === '') fail('"name" must be a non-empty string');
  if (typeof version !== 'string' || !SEMVER.test(version)) fail('"version" must be semver');
  const nodeRange =
    typeof engines === 'object' && engines !== null
      ? (engines as Record<string, unknown>).node
      : undefined;
  if (typeof nodeRange !== 'string') fail('"engines.node" is required');
  const commit = env.GIT_COMMIT?.trim() ?? '';
  return {
    name,
    version,
    commit: COMMIT.test(commit) ? commit.slice(0, 7).toLowerCase() : 'unknown',
    nodeRange,
  };
}

/** `task-tracker 1.4.0 (3f9c2ab)`, or `task-tracker 1.4.0` when the commit is unknown. */
export function describeBuild(info: BuildInfo): string {
  const base = `${info.name} ${info.version}`;
  return info.commit === 'unknown' ? base : `${base} (${info.commit})`;
}

/**
 * Does `nodeVersion` ("26.13.0" or "v26.13.0") satisfy `range`?
 * Only minimum ranges are supported: ">=24", ">=24.2", ">=24.2.1". Anything else throws an `Error`.
 */
export function satisfiesNode(range: string, nodeVersion: string): boolean {
  const wanted = /^>=(\d+)(?:\.(\d+))?(?:\.(\d+))?$/.exec(range.trim());
  const actual = /^v?(\d+)\.(\d+)\.(\d+)$/.exec(nodeVersion.trim());
  if (!wanted) throw new Error(`unsupported range "${range}"`);
  if (!actual) throw new Error(`invalid node version "${nodeVersion}"`);
  const min = [wanted[1], wanted[2] ?? '0', wanted[3] ?? '0'].map(Number);
  const have = actual.slice(1, 4).map(Number);
  for (let i = 0; i < 3; i += 1) {
    if ((have[i] ?? 0) !== (min[i] ?? 0)) return (have[i] ?? 0) > (min[i] ?? 0);
  }
  return true;
}
