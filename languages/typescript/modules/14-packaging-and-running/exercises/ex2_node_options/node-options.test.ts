import assert from 'node:assert/strict';
import { describe, test } from 'node:test';
import { auditNodeOptions, parseNodeOptions, tokenizeNodeOptions } from './node-options.ts';

describe('tokenizeNodeOptions', () => {
  test('splits on any run of whitespace', () => {
    assert.deepEqual(tokenizeNodeOptions('  --enable-source-maps \t --max-old-space-size=512\n'), [
      '--enable-source-maps',
      '--max-old-space-size=512',
    ]);
    assert.deepEqual(tokenizeNodeOptions(''), []);
  });

  test('double quotes keep spaces inside one argument', () => {
    assert.deepEqual(tokenizeNodeOptions('--title="my app" --enable-source-maps'), [
      '--title=my app',
      '--enable-source-maps',
    ]);
  });

  test('a backslash escapes the next character inside quotes', () => {
    assert.deepEqual(tokenizeNodeOptions('--title="say \\"hi\\""'), ['--title=say "hi"']);
  });

  test('an unterminated quote is an error', () => {
    assert.throws(() => tokenizeNodeOptions('--title="oops'), Error);
  });
});

describe('parseNodeOptions', () => {
  test('flags with and without values', () => {
    const flags = parseNodeOptions('--enable-source-maps --max-old-space-size=512');
    assert.equal(flags.get('--enable-source-maps'), true);
    assert.equal(flags.get('--max-old-space-size'), '512');
    assert.equal(flags.size, 2);
  });

  test('underscores in flag names are dashes, values are untouched', () => {
    const flags = parseNodeOptions('--max_old_space_size=512 --title=a_b');
    assert.equal(flags.get('--max-old-space-size'), '512');
    assert.equal(flags.get('--title'), 'a_b');
  });

  test('the last occurrence wins and stray tokens are ignored', () => {
    const flags = parseNodeOptions('--max-old-space-size=256 stray --max-old-space-size=512');
    assert.equal(flags.get('--max-old-space-size'), '512');
    assert.equal(flags.size, 1);
  });

  test('undefined means no options', () => {
    assert.equal(parseNodeOptions(undefined).size, 0);
  });
});

describe('auditNodeOptions', () => {
  test('a clean production configuration has no findings', () => {
    assert.deepEqual(
      auditNodeOptions('--enable-source-maps --max-old-space-size=384', 'production'),
      [],
    );
    assert.deepEqual(auditNodeOptions(undefined, 'production'), []);
  });

  test('an inspector in production is reported, in any spelling', () => {
    assert.deepEqual(auditNodeOptions('--inspect=0.0.0.0:9229', 'production'), [
      { code: 'inspector-enabled', flag: '--inspect' },
    ]);
    assert.deepEqual(auditNodeOptions('--inspect-brk --inspect_port=9230', 'production'), [
      { code: 'inspector-enabled', flag: '--inspect-brk' },
      { code: 'inspector-enabled', flag: '--inspect-port' },
    ]);
  });

  test('the inspector is fine outside production', () => {
    assert.deepEqual(auditNodeOptions('--inspect', 'development'), []);
    assert.deepEqual(auditNodeOptions('--inspect', undefined), []);
  });

  test('heap size must be a positive integer, everywhere', () => {
    for (const value of ['abc', '', '0', '-5', '1.5', '512mb']) {
      assert.deepEqual(
        auditNodeOptions(`--max-old-space-size=${value}`, 'development'),
        [{ code: 'invalid-heap-size', flag: '--max-old-space-size' }],
        value,
      );
    }
    assert.deepEqual(auditNodeOptions('--max-old-space-size', 'production'), [
      { code: 'invalid-heap-size', flag: '--max-old-space-size' },
    ]);
  });

  test('flags that node refuses in NODE_OPTIONS', () => {
    assert.deepEqual(auditNodeOptions('-e --print -p --eval=1', 'development'), [
      { code: 'not-allowed-in-node-options', flag: '-e' },
      { code: 'not-allowed-in-node-options', flag: '--print' },
      { code: 'not-allowed-in-node-options', flag: '-p' },
      { code: 'not-allowed-in-node-options', flag: '--eval' },
    ]);
  });

  test('findings follow the order of the flags', () => {
    assert.deepEqual(
      auditNodeOptions('--max-old-space-size=x --inspect -e', 'production').map((f) => f.code),
      ['invalid-heap-size', 'inspector-enabled', 'not-allowed-in-node-options'],
    );
  });
});
