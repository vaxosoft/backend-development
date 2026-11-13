export interface Finding {
  code: 'inspector-enabled' | 'invalid-heap-size' | 'not-allowed-in-node-options';
  flag: string;
}

/**
 * Splits a NODE_OPTIONS string into arguments. Arguments are separated by whitespace;
 * a double-quoted section may contain spaces (`--title="my app"` -> `--title=my app`)
 * and `\"` inside quotes is a literal quote. Unterminated quotes throw an `Error`.
 */
export function tokenizeNodeOptions(_text: string): string[] {
  throw new Error('TODO');
}

/**
 * Parses NODE_OPTIONS into a map from flag name to value (`true` for flags without a value).
 * Only `--name=value` and `--name` forms are understood; tokens that do not start with `-` are ignored.
 * Underscores in `--` flag names are the same as dashes (`--max_old_space_size` is `--max-old-space-size`);
 * names are stored with dashes, values are kept as written. The last occurrence of a flag wins.
 */
export function parseNodeOptions(_text: string | undefined): Map<string, string | true> {
  throw new Error('TODO');
}

/**
 * Reports problems in a NODE_OPTIONS value, in the order the flags appear:
 *
 * - `inspector-enabled`: `--inspect`, `--inspect-brk`, `--inspect-wait` or `--inspect-port`, only when
 *   `nodeEnv` is "production" (an open debugger port is remote code execution).
 * - `invalid-heap-size`: `--max-old-space-size` whose value is not a positive integer (any environment).
 * - `not-allowed-in-node-options`: `--eval`, `-e`, `--print` and `-p`: node refuses them (any environment).
 */
export function auditNodeOptions(
  _text: string | undefined,
  _nodeEnv: string | undefined,
): Finding[] {
  throw new Error('TODO');
}
