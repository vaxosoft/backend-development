# Learning site

A static site (Astro + Starlight) generated from the markdown and source files in this repo. The repo stays the single source of truth; `src/content/docs` and `src/generated` are regenerated on every build and are not committed.

## What it adds on top of the markdown

- Sidebar navigation for 14 modules and 7 projects per language, full-text search, dark mode
- Language switcher on every lesson (same topic in Go, Rust and C#) and a **Compare** page per topic with tabs for approaches, pitfalls and exercises
- **Progress tracking** in the browser (module/project completion, learning-outcome checkboxes, sidebar ticks, "continue where you left off"). Stored in `localStorage` only
- A **code viewer**: every example, exercise, solution and project file has a highlighted page with breadcrumbs. Exercise pages have a collapsed "Show the reference solution" panel

## Run locally

Requires Node 22.23 or newer (Astro 7).

```bash
cd site
npm install
npm run dev        # regenerates content, serves http://localhost:4321
```

`npm run build` writes the static site to `site/dist` (`npm run preview` serves it).

## Deploy to GitHub Pages

1. Push the repo to GitHub with the default branch named `main`.
2. In the repository settings, open **Pages** and set **Source** to **GitHub Actions**.
3. Push to `main`. [.github/workflows/pages.yml](../.github/workflows/pages.yml) builds the site and publishes it at `https://<owner>.github.io/<repo>/`.

The workflow sets `BASE_PATH`, `SITE_URL` and `REPO_URL` for you. To build for a project site by hand:

```bash
BASE_PATH=/<repo> SITE_URL=https://<owner>.github.io REPO_URL=https://github.com/<owner>/<repo> npm run build
```

For a user site or custom domain served from the root, leave `BASE_PATH` empty.

## How it works

- [scripts/generate.mjs](./scripts/generate.mjs) walks `languages/*` and `framework/`, rewrites relative links to site routes, and emits lesson pages, code pages, folder pages, compare pages and the sidebar. Set `STRICT=1` to fail on unresolved links (the CI build does).
- [public/learn.js](./public/learn.js) is the client-side layer; [src/styles/learn.css](./src/styles/learn.css) styles it.
- Adding a module or project: add it to the `TOPICS` or `PROJECTS` table at the top of `generate.mjs`. New files inside existing modules need no change.
