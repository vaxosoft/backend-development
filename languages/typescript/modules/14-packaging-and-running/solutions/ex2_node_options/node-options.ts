export interface Finding {
  code: 'inspector-enabled' | 'invalid-heap-size' | 'not-allowed-in-node-options';
  flag: string;
}

/**
 * Splits a NODE_OPTIONS string into arguments. Arguments are separated by whitespace;
 * a double-quoted section may contain spaces (`--title="my app"` -> `--title=my app`)
 * and `\"` inside quotes is a literal quote. Unterminated quotes throw an `Error`.
 */
export function tokenizeNodeOptions(text: string): string[] {
  const tokens: string[] = [];
  let current = '';
  let started = false;
  let quoted = false;
  for (let i = 0; i < text.length; i += 1) {
    const char = text.charAt(i);
    if (quoted) {
      if (char === '\\' && i + 1 < text.length) {
        i += 1;
        current += text.charAt(i);
      } else if (char === '"') {
        quoted = false;
      } else {
        current += char;
      }
    } else if (char === '"') {
      quoted = true;
      started = true;
    } else if (/\s/.test(char)) {
      if (started) tokens.push(current);
      current = '';
      started = false;
    } else {
      current += char;
      started = true;
    }
  }
  if (quoted) throw new Error('unterminated quote in NODE_OPTIONS');
  if (started) tokens.push(current);
  return tokens;
}

/**
 * Parses NODE_OPTIONS into a map from flag name to value (`true` for flags without a value).
 * Only `--name=value` and `--name` forms are understood; tokens that do not start with `-` are ignored.
 * Underscores in `--` flag names are the same as dashes (`--max_old_space_size` is `--max-old-space-size`);
 * names are stored with dashes, values are kept as written. The last occurrence of a flag wins.
 */
export function parseNodeOptions(text: string | undefined): Map<string, string | true> {
  const flags = new Map<string, string | true>();
  for (const token of tokenizeNodeOptions(text ?? '')) {
    if (!token.startsWith('-')) continue;
    const equals = token.indexOf('=');
    const rawName = equals === -1 ? token : token.slice(0, equals);
    const name = rawName.startsWith('--') ? rawName.replaceAll('_', '-') : rawName;
    flags.set(name, equals === -1 ? true : token.slice(equals + 1));
  }
  return flags;
}

const INSPECTOR_FLAGS = new Set(['--inspect', '--inspect-brk', '--inspect-wait', '--inspect-port']);
const EVAL_FLAGS = new Set(['--eval', '-e', '--print', '-p']);

/**
 * Reports problems in a NODE_OPTIONS value, in the order the flags appear:
 *
 * - `inspector-enabled`: `--inspect`, `--inspect-brk`, `--inspect-wait` or `--inspect-port`, only when
 *   `nodeEnv` is "production" (an open debugger port is remote code execution).
 * - `invalid-heap-size`: `--max-old-space-size` whose value is not a positive integer (any environment).
 * - `not-allowed-in-node-options`: `--eval`, `-e`, `--print` and `-p`: node refuses them (any environment).
 */
export function auditNodeOptions(text: string | undefined, nodeEnv: string | undefined): Finding[] {
  const findings: Finding[] = [];
  for (const [flag, value] of parseNodeOptions(text)) {
    if (INSPECTOR_FLAGS.has(flag) && nodeEnv === 'production') {
      findings.push({ code: 'inspector-enabled', flag });
    } else if (flag === '--max-old-space-size' && !/^[1-9]\d*$/.test(String(value))) {
      findings.push({ code: 'invalid-heap-size', flag });
    } else if (EVAL_FLAGS.has(flag)) {
      findings.push({ code: 'not-allowed-in-node-options', flag });
    }
  }
  return findings;
}
