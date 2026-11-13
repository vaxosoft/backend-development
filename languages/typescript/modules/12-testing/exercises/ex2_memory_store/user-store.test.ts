import assert from 'node:assert/strict';
import { describe, test } from 'node:test';
import {
  ConflictError,
  createMemoryUserStore,
  type User,
  type UserStore,
  ValidationError,
} from './user-store.ts';

type Check = (make: () => UserStore) => void;

/**
 * The contract as data: named checks that take a factory. The same checks can be pointed at any
 * implementation (memory today, SQLite tomorrow) and at deliberately broken ones.
 */
const contract: Record<string, Check> = {
  'assigns ids from 1 and never reuses them'(make) {
    const store = make();
    const a = store.create({ name: 'Ann', email: 'ann@x.io' });
    const b = store.create({ name: 'Bob', email: 'bob@x.io' });
    assert.deepEqual([a.id, b.id], [1, 2]);

    assert.equal(store.delete(b.id), true);
    assert.equal(store.create({ name: 'Cy', email: 'cy@x.io' }).id, 3);
  },

  'trims input and keeps the email case'(make) {
    const user = make().create({ name: '  Ann  ', email: ' Ann@X.io ' });
    assert.deepEqual(user, { id: 1, name: 'Ann', email: 'Ann@X.io' });
  },

  'validates name and email, naming the fields'(make) {
    const store = make();
    const fieldsOf = (input: { name: string; email: string }) => {
      try {
        store.create(input);
      } catch (err) {
        assert.ok(err instanceof ValidationError, 'expected ValidationError');
        return Object.keys(err.fields).sort();
      }
      return [];
    };

    assert.deepEqual(fieldsOf({ name: '   ', email: 'a@x.io' }), ['name']);
    assert.deepEqual(fieldsOf({ name: 'x'.repeat(101), email: 'a@x.io' }), ['name']);
    assert.deepEqual(fieldsOf({ name: 'Ann', email: 'no-at-sign' }), ['email']);
    assert.deepEqual(fieldsOf({ name: '', email: 'nope' }), ['email', 'name']);
    assert.deepEqual(store.list(), []);
  },

  'emails are unique ignoring case'(make) {
    const store = make();
    store.create({ name: 'Ann', email: 'ann@x.io' });

    assert.throws(() => store.create({ name: 'Other', email: 'ANN@x.io' }), ConflictError);
    assert.equal(store.list().length, 1);
  },

  'findByEmail ignores case and returns undefined when absent'(make) {
    const store = make();
    const ann = store.create({ name: 'Ann', email: 'Ann@x.io' });

    assert.deepEqual(store.findByEmail('ANN@X.IO'), ann);
    assert.equal(store.findByEmail('nobody@x.io'), undefined);
  },

  'returns copies, never live objects'(make) {
    const store = make();
    const created = store.create({ name: 'Ann', email: 'ann@x.io' });
    created.name = 'Hacked';
    (store.get(created.id) as User).name = 'Hacked again';
    (store.list()[0] as User).name = 'Hacked a third time';

    assert.equal(store.get(created.id)?.name, 'Ann');
  },

  'lists by id ascending'(make) {
    const store = make();
    for (const n of ['c', 'a', 'b']) store.create({ name: n, email: `${n}@x.io` });
    assert.deepEqual(
      store.list().map((u) => u.name),
      ['c', 'a', 'b'],
    );
  },

  'rename validates and reports unknown ids'(make) {
    const store = make();
    const { id } = store.create({ name: 'Ann', email: 'ann@x.io' });

    assert.equal(store.rename(id, '  Anna ')?.name, 'Anna');
    assert.throws(() => store.rename(id, ''), ValidationError);
    assert.equal(store.get(id)?.name, 'Anna');
    assert.equal(store.rename(99, 'Nobody'), undefined);
  },

  'delete reports whether something was removed'(make) {
    const store = make();
    const { id } = store.create({ name: 'Ann', email: 'ann@x.io' });

    assert.equal(store.delete(id), true);
    assert.equal(store.delete(id), false);
    assert.equal(store.get(id), undefined);
    assert.equal(store.findByEmail('ann@x.io'), undefined);
  },
};

describe('UserStore contract: createMemoryUserStore', () => {
  for (const [name, check] of Object.entries(contract)) {
    test(name, () => check(createMemoryUserStore));
  }
});

// A contract is only useful if it can fail. These implementations are wrong in one way each.
function brokenStore(flaw: 'case-sensitive' | 'reuses-ids' | 'leaky'): UserStore {
  const users: User[] = [];
  let nextId = 1;
  const same = (a: string, b: string) =>
    flaw === 'case-sensitive' ? a === b : a.toLowerCase() === b.toLowerCase();
  const out = (u: User): User => (flaw === 'leaky' ? u : { ...u });
  return {
    create({ name, email }) {
      if (name.trim() === '' || name.length > 100) throw new ValidationError({ name: 'bad' });
      if (!email.includes('@')) throw new ValidationError({ email: 'bad' });
      if (users.some((u) => same(u.email, email.trim()))) throw new ConflictError('dup');
      const id = flaw === 'reuses-ids' ? users.length + 1 : nextId++;
      const user = { id, name: name.trim(), email: email.trim() };
      users.push(user);
      return out(user);
    },
    get(id) {
      const user = users.find((u) => u.id === id);
      return user && out(user);
    },
    findByEmail: (email) => users.find((u) => same(u.email, email)),
    list: () => users.map(out),
    rename(id, name) {
      const user = users.find((u) => u.id === id);
      if (!user) return undefined;
      if (name.trim() === '') throw new ValidationError({ name: 'bad' });
      user.name = name.trim();
      return out(user);
    },
    delete(id) {
      const index = users.findIndex((u) => u.id === id);
      if (index < 0) return false;
      users.splice(index, 1);
      return true;
    },
  };
}

describe('the contract catches broken implementations', () => {
  for (const flaw of ['case-sensitive', 'reuses-ids', 'leaky'] as const) {
    test(flaw, () => {
      const failures = Object.values(contract).filter((check) => {
        try {
          check(() => brokenStore(flaw));
          return false;
        } catch {
          return true;
        }
      });
      assert.ok(failures.length >= 1, `no check noticed the "${flaw}" flaw`);
    });
  }
});
