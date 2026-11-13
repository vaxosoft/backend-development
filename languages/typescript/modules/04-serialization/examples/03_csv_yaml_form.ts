// Other formats: CSV and YAML need libraries; form-encoded data is built in via URLSearchParams.
import { parse as parseCsv } from 'csv-parse/sync';
import { stringify as stringifyCsv } from 'csv-stringify/sync';
import { parse as parseYaml, stringify as stringifyYaml } from 'yaml';

// CSV: every cell is a string. Quoting of commas, quotes and newlines is handled for you.
const rows = [
  { id: 1, title: 'Write docs, then review', tags: 'docs|p1' },
  { id: 2, title: 'Say "hello"', tags: '' },
];
const csv = stringifyCsv(rows, { header: true });
process.stdout.write(csv);

const back = parseCsv(csv, { columns: true, skip_empty_lines: true }) as Record<string, string>[];
console.log(back[0]?.title, typeof back[0]?.id);

// With `cast` the parser can convert columns, but validating afterwards (zod) is more explicit.
const typed = parseCsv('id,done\n1,true\n2,false\n', {
  columns: true,
  cast: (value, ctx) => {
    if (ctx.header) return value;
    return ctx.column === 'id' ? Number(value) : value === 'true';
  },
});
console.log(typed);

// YAML: a superset of JSON that is friendlier for config files. Dates stay strings by default.
const doc = parseYaml(`
server:
  host: 127.0.0.1
  port: 8080
  tags: [a, b]
due: 2030-01-31
`);
console.log(doc);
console.log(stringifyYaml({ title: 'Write docs', tags: ['docs', 'p1'], due_date: null }).trimEnd());

// application/x-www-form-urlencoded: what an HTML <form> posts.
const form = new URLSearchParams('title=Buy+milk&tag=home&tag=p1&empty=');
console.log(
  form.get('title'),
  form.getAll('tag'),
  JSON.stringify(form.get('empty')),
  form.get('x'),
);
console.log(Object.fromEntries(form)); // keeps only the last value of a repeated key
console.log(new URLSearchParams({ q: 'a&b=c', page: '2' }).toString());
