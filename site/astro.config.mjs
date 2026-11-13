import fs from 'node:fs';
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

// Deployment settings (GitHub Pages project sites live under /<repo>/).
const SITE_URL = process.env.SITE_URL || undefined;
let base = (process.env.BASE_PATH || '').trim().replace(/\/+$/, '');
if (base && !base.startsWith('/')) base = `/${base}`;
const repoUrl = (process.env.REPO_URL || '').replace(/\/+$/, '');

const sidebarFile = new URL('./src/generated/sidebar.json', import.meta.url);
const sidebar = fs.existsSync(sidebarFile) ? JSON.parse(fs.readFileSync(sidebarFile, 'utf8')) : [];

export default defineConfig({
  site: SITE_URL,
  base: base || undefined,
  integrations: [
    starlight({
      title: 'Backend Learning Paths',
      description: 'Learn backend development in Go, Rust, C#, TypeScript and Python.',
      customCss: ['./src/styles/learn.css'],
      head: [{ tag: 'script', attrs: { src: `${base}/learn.js`, defer: true } }],
      social: repoUrl ? [{ icon: 'github', label: 'GitHub', href: repoUrl }] : [],
      sidebar,
      pagination: true,
    }),
  ],
});
