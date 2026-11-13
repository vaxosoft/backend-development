// Identifiers, randomness, hashing, regular expressions and string templates.
import { createHash, createHmac, randomBytes, randomInt, randomUUID } from 'node:crypto';

// 1. UUID v4 from the CSPRNG. The value changes on every run, so print only its shape.
const id = randomUUID();
console.log(
  'uuid shape ok:',
  /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/.test(id),
);

// 2. randomInt(min, max) is unbiased and max is exclusive; Math.random() is not for secrets.
const roll = randomInt(1, 7);
console.log('dice roll in 1..6:', roll >= 1 && roll <= 6);
const token = randomBytes(16).toString('base64url');
console.log('token length:', token.length);

// 3. Hashing: the same input always gives the same digest. Great for ETags, never for passwords.
const body = JSON.stringify({ id: 1, title: 'Write docs' });
const etag = createHash('sha256').update(body).digest('hex');
console.log('sha256:', etag);
console.log('sha1 :', createHash('sha1').update('abc').digest('hex'));

// 4. HMAC signs a message with a secret key (webhooks, signed URLs).
const signature = createHmac('sha256', 'shared-secret').update(body).digest('hex');
console.log('hmac :', signature.slice(0, 16));

// 5. Regular expressions with named groups; exec/matchAll return `null`/iterators, not booleans.
const dueDate = /^(?<year>\d{4})-(?<month>0[1-9]|1[0-2])-(?<day>0[1-9]|[12]\d|3[01])$/;
const match = dueDate.exec('2030-01-31');
console.log('groups:', { ...match?.groups });
console.log('no match:', dueDate.exec('31/01/2030'));

const text = 'Fix #12 and #7, then close #101';
const refs = [...text.matchAll(/#(?<n>\d+)/g)].map((m) => Number(m.groups?.n));
console.log('issue refs:', refs);
console.log('replace:', text.replace(/#(?<n>\d+)/g, 'issue-$<n>'));

// 6. A global (/g) regex object keeps state in lastIndex: reusing it with test() is a trap.
const sticky = /a/g;
console.log('test twice with /g:', sticky.test('a'), sticky.test('a'));

// 7. Text templating: template literals, tagged templates for safe output.
const escapeHtml = (s: string) => s.replace(/[&<>"']/g, (c) => `&#${c.charCodeAt(0)};`);
function html(strings: TemplateStringsArray, ...values: unknown[]): string {
  return strings.reduce(
    (out, chunk, i) => out + chunk + (i < values.length ? escapeHtml(String(values[i])) : ''),
    '',
  );
}
const title = '<script>alert("x")</script>';
console.log(html`<li>${title}</li>`);
console.log(`Task ${1}: ${'Write docs'.toUpperCase().padEnd(12, '.')}|`);
