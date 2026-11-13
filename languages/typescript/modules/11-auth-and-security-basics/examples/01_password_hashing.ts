import { timingSafeEqual } from 'node:crypto';
import { hashPassword, verifyPassword } from './_shared.ts';

const stored = await hashPassword('correct horse battery staple');
const [scheme, cost, salt, hash] = stored.split('$');
console.log('format:', scheme, cost, `salt=${salt?.length} chars`, `hash=${hash?.length} chars`);

console.log('right password:', await verifyPassword('correct horse battery staple', stored));
console.log('wrong password:', await verifyPassword('Correct horse battery staple', stored));
console.log('malformed hash:', await verifyPassword('anything', 'not-a-hash'));

// A fresh random salt makes every hash different, even for the same password.
const again = await hashPassword('correct horse battery staple');
console.log('same password, different hashes:', stored !== again);
console.log('both verify:', await verifyPassword('correct horse battery staple', again));

try {
  timingSafeEqual(Buffer.from('short'), Buffer.from('much longer'));
} catch (err) {
  console.log('timingSafeEqual on unequal lengths throws:', (err as { code?: string }).code);
}
