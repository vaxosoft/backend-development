// Generates site/src/content/docs from the repository markdown and source files.
// The repo stays the single source of truth; nothing generated is committed.
//
// Env: BASE_PATH (e.g. /web-development for GitHub Pages), REPO_URL (https://github.com/o/r), STRICT=1 (fail on unresolved links)
import fs from 'node:fs';
import path from 'node:path';

const SITE_DIR = path.resolve(import.meta.dirname, '..');
const ROOT = path.resolve(SITE_DIR, '..');
const OUT = path.join(SITE_DIR, 'src/content/docs');
const GEN = path.join(SITE_DIR, 'src/generated');

let BASE = (process.env.BASE_PATH ?? '').trim().replace(/\/+$/, '');
if (BASE && !BASE.startsWith('/')) BASE = `/${BASE}`;
const REPO_URL = (process.env.REPO_URL ?? '').replace(/\/+$/, '');

const LANGS = [
  { id: 'go', name: 'Go', blurb: 'Go 1.25+, net/http, database/sql, log/slog. Small standard library, goroutines, single static binary.' },
  { id: 'rust', name: 'Rust', blurb: 'Rust 2024, tokio, axum, sqlx, serde. Ownership and the type system do the heavy lifting.' },
  { id: 'csharp', name: 'C#', blurb: 'C# 14 on .NET 10, ASP.NET Core minimal APIs, ADO.NET and EF Core. Batteries-included platform.' },
  { id: 'typescript', name: 'TypeScript', blurb: 'TypeScript 7 on Node 24+, Fastify, zod, node:test. Runs .ts directly with no build step.' },
  { id: 'python', name: 'Python', blurb: 'Python 3.14, FastAPI, pydantic, sqlite3, pytest. Fully type-hinted and mypy-strict.' },
];
const NAMES = LANGS.map((l) => l.name);
const joinNames = (names) => (names.length > 1 ? `${names.slice(0, -1).join(', ')} and ${names.at(-1)}` : names[0] ?? '');
const NUM_WORDS = ['zero', 'one', 'two', 'three', 'four', 'five', 'six', 'seven'];
const TOPICS = {
  '01-language-fundamentals': 'Language fundamentals',
  '02-http-server': 'HTTP server',
  '03-rest-api-design': 'REST API design',
  '04-serialization': 'Serialization',
  '05-database-access': 'Database access',
  '06-concurrency': 'Concurrency',
  '07-http-client': 'HTTP client',
  '08-file-and-stream-io': 'File and stream I/O',
  '09-configuration': 'Configuration',
  '10-logging-and-diagnostics': 'Logging and diagnostics',
  '11-auth-and-security-basics': 'Auth and security basics',
  '12-testing': 'Testing',
  '13-time-and-stdlib-utilities': 'Time and standard library utilities',
  '14-packaging-and-running': 'Packaging and running',
};
const PROJECTS = {
  'P1-hello-http': 'Hello HTTP',
  'P2-tasks-crud-api': 'Tasks CRUD API',
  'P3-file-service': 'File service',
  'P4-api-aggregator': 'API aggregator',
  'P5-url-shortener': 'URL shortener',
  'P6-job-worker-pool': 'Job worker pool',
  'P7-tested-service-docker-ci': 'Tested service, Docker, CI',
};

const SKIP_DIRS = new Set(['bin', 'obj', 'target', 'node_modules', 'dist', '.git', '.vs', '.idea', '.venv', '__pycache__', '.mypy_cache', '.ruff_cache', '.pytest_cache']);
const SKIP_FILES = new Set(['go.sum', 'package-lock.json']);
const EXT_LANG = {
  go: 'go', rs: 'rust', cs: 'csharp', ts: 'ts', mts: 'ts', py: 'python', mjs: 'js', cjs: 'js', ini: 'ini', cfg: 'ini', env: 'ini', csproj: 'xml', props: 'xml', toml: 'toml', lock: 'toml', yml: 'yaml', yaml: 'yaml',
  json: 'json', jsonl: 'json', md: 'md', html: 'html', css: 'css', js: 'js', sql: 'sql', sh: 'bash', txt: 'text', csv: 'text',
  xml: 'xml', mod: 'go', tmpl: 'text', dockerignore: 'text',
};
const MAX_BYTES = 500 * 1024;

const posix = path.posix;
const read = (p) => fs.readFileSync(path.join(ROOT, p), 'utf8').replace(/\r\n/g, '\n');
const exists = (p) => fs.existsSync(path.join(ROOT, p));
const warnings = [];
const warn = (m) => warnings.push(m);
const yaml = (s) => JSON.stringify(s);
const attr = (s) => String(s).replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;').replace(/>/g, '&gt;');

// ---------------------------------------------------------------- inventory
const files = new Set(); // repo-relative posix paths that get a code page
const dirs = new Set(); // directories that contain at least one such file
const lessonPages = new Map(); // repo path -> slug

function fileLang(rel) {
  const base = posix.basename(rel);
  if (/^Dockerfile/i.test(base)) return 'dockerfile';
  const ext = base.includes('.') ? base.split('.').pop() : '';
  return EXT_LANG[ext] ?? null;
}

function walk(rel) {
  for (const e of fs.readdirSync(path.join(ROOT, rel), { withFileTypes: true })) {
    const r = posix.join(rel, e.name);
    if (e.isDirectory()) {
      if (!SKIP_DIRS.has(e.name)) walk(r);
      continue;
    }
    if (SKIP_FILES.has(e.name)) continue;
    if (!fileLang(r)) {
      warn(`skipped (unknown type): ${r}`);
      continue;
    }
    const buf = fs.readFileSync(path.join(ROOT, r));
    if (buf.length > MAX_BYTES || buf.includes(0)) {
      warn(`skipped (large/binary): ${r}`);
      continue;
    }
    files.add(r);
  }
}

const isInfra = (rel) => /\/(00-[^/]+|P0-[^/]+)(\/|$)/.test(rel);

for (const l of LANGS) {
  const root = `languages/${l.id}`;
  walk(root);
  lessonPages.set(`${root}/README.md`, l.id);
  lessonPages.set(`${root}/PROFILE.md`, `${l.id}/profile`);
  for (const t of Object.keys(TOPICS)) lessonPages.set(`${root}/modules/${t}/README.md`, `${l.id}/modules/${t}`);
  for (const p of Object.keys(PROJECTS)) lessonPages.set(`${root}/projects/${p}/README.md`, `${l.id}/projects/${p}`);
}
for (const f of [...files]) {
  if (isInfra(f) || lessonPages.has(f)) files.delete(f);
}
const frameworkPages = new Map();
for (const f of fs.readdirSync(path.join(ROOT, 'framework'))) {
  if (f.endsWith('.md')) frameworkPages.set(`framework/${f}`, `framework/${f.replace(/\.md$/, '')}`);
}
frameworkPages.set('backend-core-topics.md', 'framework/backend-core-topics');
frameworkPages.set('README.md', '');
for (const f of files) {
  for (let d = posix.dirname(f); d !== '.' && d !== 'languages'; d = posix.dirname(d)) dirs.add(d);
}

const lessonDirs = new Map(); // directory of a module/project lesson -> slug
for (const [p, slug] of lessonPages) {
  if (/\/modules\/|\/projects\//.test(p)) lessonDirs.set(posix.dirname(p), slug);
}

const langOf = (rel) => rel.split('/')[1];
const url = (slug) => `${BASE}/${slug ? `${slug}/` : ''}`;

function pageFor(rel) {
  if (!rel || rel === '.') return '';
  if (lessonPages.has(rel)) return lessonPages.get(rel);
  if (frameworkPages.has(rel)) return frameworkPages.get(rel);
  if (rel === 'framework') return 'framework/README';
  if (lessonDirs.has(rel)) return lessonDirs.get(rel);
  if (/^languages\/[^/]+$/.test(rel) && LANGS.some((l) => l.id === rel.split('/')[1])) return rel.split('/')[1];
  if (files.has(rel) || dirs.has(rel)) {
    // a file literally named index.html would collide with the generated index.html of its folder page
    const segs = rel.split('/').slice(2).map((s) => (s === 'index.html' ? '_index.html' : s));
    return `${langOf(rel)}/code/${segs.join('/')}`;
  }
  return null;
}

// ---------------------------------------------------------------- markdown helpers
function mapOutsideCode(md, fn) {
  const out = [];
  let fence = null;
  for (const line of md.split('\n')) {
    const m = line.match(/^\s*(`{3,}|~{3,})/);
    if (fence) {
      if (m && m[1][0] === fence[0] && m[1].length >= fence.length) fence = null;
      out.push(line);
    } else if (m) {
      fence = m[1];
      out.push(line);
    } else {
      out.push(line.split(/(`+[^`]*`+)/).map((seg, i) => (i % 2 ? seg : fn(seg))).join(''));
    }
  }
  return out.join('\n');
}

function resolveLink(href, src) {
  if (/^([a-z][a-z0-9+.-]*:|#|\/)/i.test(href)) return href;
  if (/[<>]/.test(href)) return null; // template placeholder, not a real link
  const [pathPart, ...rest] = href.split(/(?=[#?])/);
  const suffix = rest.join('');
  let target;
  try {
    target = posix.normalize(posix.join(posix.dirname(src), decodeURI(pathPart))).replace(/\/$/, '');
  } catch {
    target = null;
  }
  if (!target || target.startsWith('..')) return null;
  const slug = pageFor(target);
  const frag = suffix.startsWith('#') ? suffix : '';
  if (slug !== null) return url(slug) + frag;
  warn(`unresolved link in ${src}: ${href}`);
  return REPO_URL ? `${REPO_URL}/blob/main/${target}${frag}` : null;
}

function rewriteLinks(md, src) {
  return mapOutsideCode(md, (seg) =>
    seg
      .replace(/(!?)\[((?:[^[\]]|\[[^\]]*\])*)\]\(([^)\s]+)((?:\s+"[^"]*")?)\)/g, (all, bang, text, href, title) => {
        const r = resolveLink(href, src);
        if (r === null) return bang ? '' : text;
        return `${bang}[${text}](${r}${title})`;
      })
      .replace(/\b(href|src)="([^"]+)"/g, (all, a, href) => {
        const r = resolveLink(href, src);
        return r === null ? '' : `${a}="${r}"`;
      }),
  );
}

function splitH1(md) {
  const lines = md.split('\n');
  const i = lines.findIndex((l) => /^# /.test(l));
  if (i < 0) return { title: '', body: md };
  const title = lines[i].replace(/^# /, '').trim();
  lines.splice(i, 1);
  return { title, body: lines.join('\n').replace(/^\n+/, '') };
}

function sections(md) {
  const res = [];
  let cur = null;
  let fence = null;
  for (const line of md.split('\n')) {
    const m = line.match(/^\s*(`{3,}|~{3,})/);
    if (fence) {
      if (m && m[1][0] === fence[0] && m[1].length >= fence.length) fence = null;
    } else if (m) fence = m[1];
    const h = !fence && !m && line.match(/^## (.*)$/);
    if (h) {
      cur = { heading: h[1].trim(), lines: [] };
      res.push(cur);
    } else if (cur) cur.lines.push(line);
  }
  return res.map((s) => ({ heading: s.heading, text: s.lines.join('\n').trim() }));
}

function write(slug, frontmatter, body) {
  const file = path.join(OUT, slug === '' ? 'index.md' : `${slug}.md`);
  fs.mkdirSync(path.dirname(file), { recursive: true });
  const fm = Object.entries(frontmatter)
    .map(([k, v]) => `${k}: ${typeof v === 'string' ? yaml(v) : JSON.stringify(v)}`)
    .join('\n');
  fs.writeFileSync(file, `---\n${fm}\nslug: ${yaml(slug === '' ? 'index' : slug)}\n---\n\n${body.trim()}\n`);
}

fs.rmSync(OUT, { recursive: true, force: true });

// ---------------------------------------------------------------- curriculum data (progress board, sidebar)
const curriculum = {};
for (const l of LANGS) {
  const modules = Object.entries(TOPICS).map(([folder, title]) => {
    const src = `languages/${l.id}/modules/${folder}/README.md`;
    const time = (exists(src) && read(src).match(/Estimated time:\s*([^|\n]+)/)?.[1].trim()) || '';
    return { id: `${l.id}/${folder}`, title: `${folder.slice(0, 2)} ${title}`, url: url(`${l.id}/modules/${folder}`), time };
  });
  const projects = Object.entries(PROJECTS).map(([folder, title]) => ({
    id: `${l.id}/${folder}`,
    title: `${folder.slice(0, 2)} ${title}`,
    url: url(`${l.id}/projects/${folder}`),
  }));
  curriculum[l.id] = { name: l.name, modules, projects };
}
const board = (lang, cls = '') =>
  `<div class="progress-board${cls}" data-lang="${lang}" data-curriculum="${attr(JSON.stringify(curriculum[lang]))}"></div>`;

// ---------------------------------------------------------------- lessons
function langSwitch(kind, folder, current) {
  return LANGS.map((l) =>
    l.id === current ? `<span class="current">${l.name}</span>` : `<a href="${url(`${l.id}/${kind}/${folder}`)}">${l.name}</a>`,
  ).join('');
}

function browseLinks(lessonDir) {
  const names = ['examples', 'exercises', 'solutions', '_solutions', 'starter', 'tests', 'solution', '_solution'];
  return names
    .filter((n) => dirs.has(`${lessonDir}/${n}`))
    .map((n) => `<a href="${url(pageFor(`${lessonDir}/${n}`))}">${n.replace(/^_/, '')}</a>`)
    .join('');
}

function lessonBar(kind, folder, lang, lessonDir, bottom = false) {
  const id = `${lang}/${folder}`;
  const label = kind === 'modules' ? 'module' : 'project';
  const done = `<button type="button" class="done-toggle" data-done-toggle="${id}" data-label="${label}">Mark ${label} complete</button>`;
  if (bottom) return `<div class="lesson-bar bottom">${done}</div>`;
  const cmp = kind === 'modules' ? `<a class="compare" href="${url(`compare/${folder}`)}">Compare languages</a>` : '';
  return `<div class="lesson-bar" data-lesson="${id}" data-title="${attr(TOPICS[folder] ?? PROJECTS[folder] ?? folder)}">
<div class="row"><span class="label">Also in</span>${langSwitch(kind, folder, lang)}${cmp}</div>
<div class="row"><span class="label">Browse code</span>${browseLinks(lessonDir)}</div>
<div class="row">${done}</div>
</div>`;
}

for (const l of LANGS) {
  const root = `languages/${l.id}`;

  {
    const src = `${root}/README.md`;
    const { title, body } = splitH1(read(src));
    const md = body.replace(/## Progress checklist[\s\S]*?(?=\n## |$)/, `## Your progress\n\nSaved in this browser only.\n\n${board(l.id)}\n`);
    write(l.id, { title, description: l.blurb, tableOfContents: false }, rewriteLinks(md, src));
  }

  {
    const src = `${root}/PROFILE.md`;
    const { title, body } = splitH1(read(src));
    write(`${l.id}/profile`, { title: title || `${l.name} profile` }, rewriteLinks(body, src));
  }

  for (const [kind, table] of [['modules', TOPICS], ['projects', PROJECTS]]) {
    for (const folder of Object.keys(table)) {
      const src = `${root}/${kind}/${folder}/README.md`;
      if (!exists(src)) {
        warn(`missing lesson: ${src}`);
        continue;
      }
      const { title, body } = splitH1(read(src));
      write(
        `${l.id}/${kind}/${folder}`,
        { title: title || table[folder], description: `${l.name}: ${table[folder]}` },
        `${lessonBar(kind, folder, l.id, `${root}/${kind}/${folder}`)}\n\n${rewriteLinks(body, src)}\n\n${lessonBar(kind, folder, l.id, '', true)}`,
      );
    }
  }
}

// ---------------------------------------------------------------- code pages
const SOLUTION_DIRS = (lang) => (lang === 'go' ? { exercises: '_solutions', starter: '_solution' } : { exercises: 'solutions', starter: 'solution' });

function counterpart(rel) {
  const parts = rel.split('/');
  for (const [from, to] of Object.entries(SOLUTION_DIRS(langOf(rel)))) {
    const i = parts.indexOf(from);
    if (i >= 2) {
      const cand = [...parts.slice(0, i), to, ...parts.slice(i + 1)].join('/');
      if (files.has(cand)) return { kind: 'solution', path: cand };
    }
    const j = parts.indexOf(to);
    if (j >= 2) {
      const cand = [...parts.slice(0, j), from, ...parts.slice(j + 1)].join('/');
      if (files.has(cand)) return { kind: 'starter', path: cand };
    }
  }
  return null;
}

const fenceFor = (text) => '`'.repeat(Math.max(4, ...[...text.matchAll(/`+/g)].map((m) => m[0].length + 1)));
const codeBlock = (rel, text) => {
  const f = fenceFor(text);
  return `${f}${fileLang(rel)} title="${posix.basename(rel)}"\n${text.replace(/\n$/, '')}\n${f}`;
};

function crumbs(rel, isDir) {
  const parts = rel.split('/');
  const out = [];
  for (let i = 1; i < parts.length; i++) {
    const p = parts.slice(0, i + 1).join('/');
    const last = i === parts.length - 1;
    const label = i === 1 ? LANGS.find((l) => l.id === parts[1]).name : parts[i];
    const slug = last && !isDir ? null : pageFor(p);
    out.push(last || slug === null ? `**${label}**` : `[${label}](${url(slug)})`);
  }
  return out.join(' / ');
}

for (const f of files) {
  const other = counterpart(f);
  let extra = '';
  let banner = '';
  if (other?.kind === 'solution') {
    extra = `\n\n<details class="solution">\n<summary>Show the reference solution</summary>\n\nTry the exercise first. This is \`${other.path.split('/').slice(2).join('/')}\`.\n\n${codeBlock(other.path, read(other.path))}\n\n</details>`;
  } else if (other?.kind === 'starter') {
    banner = `\n:::caution[Reference solution]\nSpoiler ahead if you have not attempted the exercise. [Open the starter](${url(pageFor(other.path))}).\n:::\n`;
  }
  write(
    pageFor(f),
    { title: posix.basename(f), description: f, tableOfContents: false, pagefind: false, prev: false, next: false },
    `${crumbs(f, false)}\n${banner}\n${codeBlock(f, read(f))}${extra}`,
  );
}

for (const d of dirs) {
  const slug = pageFor(d);
  if (lessonDirs.has(d) || !slug.includes('/code/')) continue;
  const entries = [...files, ...dirs]
    .filter((p) => posix.dirname(p) === d)
    .map((p) => [p, dirs.has(p)])
    .sort(([a, ad], [b, bd]) => (ad === bd ? a.localeCompare(b) : ad ? -1 : 1));
  const list = entries.map(([p, isDir]) => `- ${isDir ? '📁' : '📄'} [${posix.basename(p)}${isDir ? '/' : ''}](${url(pageFor(p))})`).join('\n');
  const up = pageFor(posix.dirname(d));
  write(
    slug,
    { title: `${posix.basename(d)}/`, description: d, tableOfContents: false, pagefind: false, prev: false, next: false },
    `${crumbs(d, true)}\n\n${list}${up ? `\n\n[Up one level](${url(up)})` : ''}`,
  );
}

// ---------------------------------------------------------------- framework docs
for (const [rel, slug] of frameworkPages) {
  if (!slug) continue;
  const { title, body } = splitH1(read(rel));
  write(slug, { title: title || slug }, rewriteLinks(body, rel));
}

// ---------------------------------------------------------------- compare pages
const WANTED = [
  [/^2\./, 'How each language approaches it'],
  [/^4\./, 'Common pitfalls'],
  [/^6\./, 'Exercises'],
];
function tabs(parts) {
  const buttons = parts.map((p) => `<button type="button" data-lang="${p.lang}">${p.name}</button>`).join('');
  const panels = parts.map((p) => `<div class="lang-panel" data-lang="${p.lang}">\n\n${p.text}\n\n</div>`).join('\n\n');
  return `<div class="lang-tabs">\n<div class="lang-tab-buttons">${buttons}</div>\n\n${panels}\n\n</div>`;
}
for (const [folder, topic] of Object.entries(TOPICS)) {
  const perLang = LANGS.map((l) => {
    const src = `languages/${l.id}/modules/${folder}/README.md`;
    return exists(src) ? { l, src, secs: sections(read(src)) } : null;
  }).filter(Boolean);
  let md = `Same topic, ${NUM_WORDS[LANGS.length] ?? LANGS.length} languages. Your tab choice is remembered. Full lessons: ${perLang.map((p) => `[${p.l.name}](${url(`${p.l.id}/modules/${folder}`)})`).join(' · ')}.\n`;
  for (const [re, heading] of WANTED) {
    const parts = perLang
      .map((p) => {
        const s = p.secs.find((x) => re.test(x.heading));
        return s ? { lang: p.l.id, name: p.l.name, text: rewriteLinks(s.text, p.src) } : null;
      })
      .filter(Boolean);
    if (parts.length) md += `\n## ${heading}\n\n${tabs(parts)}\n`;
  }
  write(`compare/${folder}`, { title: `Compare: ${topic}`, description: `${topic} in ${joinNames(NAMES)}`, tableOfContents: false }, md);
}
write(
  'compare',
  { title: 'Compare languages', description: `The same backend topic in ${joinNames(NAMES)}` },
  `Pick a topic to see how ${joinNames(NAMES)} approach it side by side.\n\n| Topic | Compare | Lessons |\n|---|---|---|\n${Object.entries(TOPICS)
    .map(
      ([folder, t]) =>
        `| ${folder.slice(0, 2)} ${t} | [Compare](${url(`compare/${folder}`)}) | ${LANGS.map((l) => `[${l.name}](${url(`${l.id}/modules/${folder}`)})`).join(' · ')} |`,
    )
    .join('\n')}\n`,
);

// ---------------------------------------------------------------- progress + home
write(
  'progress',
  { title: 'Your progress', description: 'Track which modules and projects you have completed', tableOfContents: false },
  `Progress is stored in this browser only (local storage). Nothing is sent anywhere.\n\n<div class="continue-box" data-continue></div>\n\n${LANGS.map((l) => `## ${l.name}\n\n${board(l.id)}`).join('\n\n')}\n\n<button type="button" class="reset-progress">Reset all progress</button>\n`,
);

const cards = LANGS.map(
  (l) => `<div class="lang-card">
<h3>${l.name}</h3>
<p>${l.blurb}</p>
<p class="actions"><a class="primary" href="${url(l.id)}">Start learning</a><a href="${url(`${l.id}/profile`)}">Profile</a></p>
${board(l.id, ' compact')}
</div>`,
).join('\n');
write(
  '',
  {
    title: 'Backend Development Learning Paths',
    description: `Learn backend development in ${joinNames(NAMES)} with runnable examples, exercises and capstone projects.`,
    template: 'splash',
    prev: false,
    next: false,
    hero: {
      tagline: `One Task Tracker domain, ${NUM_WORDS[LANGS.length] ?? LANGS.length} languages. 14 modules and 7 capstone projects each, with runnable examples and exercises that fail until you solve them.`,
      actions: [
        { text: 'Pick a language', link: '#languages', icon: 'down-caret' },
        { text: 'Compare languages', link: url('compare'), variant: 'minimal', icon: 'right-arrow' },
      ],
    },
  },
  `<div class="continue-box" data-continue></div>

<h2 id="languages">Languages</h2>

<div class="lang-cards">
${cards}
</div>

## How it works

- Each **module** has a lesson, 4-5 runnable examples and three exercises (easy, medium, stretch) with tests.
- Each **capstone project** has requirements, a starter, a black-box test suite and a reference solution.
- Use the **language switcher** on any lesson to jump to the same topic elsewhere, or open **Compare** to read them side by side.
- Mark modules complete to track progress. See [your progress](${url('progress')}).
`,
);

// ---------------------------------------------------------------- sidebar
const sidebar = [
  {
    label: 'Start here',
    items: [
      { label: 'Home', link: '/' },
      { label: 'Your progress', slug: 'progress' },
      { label: 'Compare languages', slug: 'compare' },
    ],
  },
  ...LANGS.map((l) => ({
    label: l.name,
    collapsed: true,
    items: [
      { label: 'Overview', slug: l.id },
      { label: 'Profile', slug: `${l.id}/profile` },
      { label: 'Modules', collapsed: false, items: curriculum[l.id].modules.map((m) => ({ label: m.title, slug: m.id.replace('/', '/modules/') })) },
      { label: 'Projects', collapsed: false, items: curriculum[l.id].projects.map((m) => ({ label: m.title, slug: m.id.replace('/', '/projects/') })) },
    ],
  })),
  {
    label: 'Framework',
    collapsed: true,
    items: [...frameworkPages.values()].filter(Boolean).map((slug) => ({ label: slug.split('/')[1].replace(/-/g, ' '), slug })),
  },
];
fs.mkdirSync(GEN, { recursive: true });
fs.writeFileSync(path.join(GEN, 'sidebar.json'), JSON.stringify(sidebar, null, 2));

// ---------------------------------------------------------------- report
const unresolved = warnings.filter((w) => w.startsWith('unresolved'));
for (const w of warnings) console.warn(`[generate] ${w}`);
console.log(`[generate] ${files.size} code pages, ${dirs.size} folder pages, ${unresolved.length} unresolved links`);
if (unresolved.length && process.env.STRICT) process.exit(1);
