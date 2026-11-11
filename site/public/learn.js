// Client-side learning layer: progress tracking, language tabs, "continue" link.
// All state lives in localStorage; nothing leaves the browser.
(() => {
  const KEY = 'bl:v1';
  const root = document.documentElement;
  root.classList.add('js-tabs');

  const load = () => {
    try {
      return JSON.parse(localStorage.getItem(KEY)) || {};
    } catch {
      return {};
    }
  };
  const state = load();
  state.done ??= {};
  state.outcomes ??= {};
  const save = () => {
    try {
      localStorage.setItem(KEY, JSON.stringify(state));
    } catch {
      /* storage unavailable: progress just won't persist */
    }
  };

  const el = (tag, props = {}, ...kids) => {
    const n = Object.assign(document.createElement(tag), props);
    n.append(...kids);
    return n;
  };

  function lessonIdFromPath(pathname) {
    const m = pathname.match(/\/(go|rust|csharp)\/(?:modules|projects)\/([^/]+)\/?$/);
    return m ? `${m[1]}/${m[2]}` : null;
  }

  function setDone(id, value) {
    if (value) state.done[id] = true;
    else delete state.done[id];
    save();
    refresh();
  }

  // ---- progress boards
  function renderBoard(node) {
    const data = JSON.parse(node.dataset.curriculum);
    const items = [...data.modules, ...data.projects];
    const done = items.filter((i) => state.done[i.id]).length;
    const pct = Math.round((done / items.length) * 100);
    node.replaceChildren(
      el(
        'div',
        { className: 'board-head' },
        el('div', { className: 'bar', role: 'progressbar', ariaValueNow: String(pct), ariaValueMin: '0', ariaValueMax: '100' }, el('span', { style: `width:${pct}%` })),
        el('span', { className: 'pct' }, `${done}/${items.length} complete`),
      ),
    );
    if (node.classList.contains('compact')) return;
    const group = (title, list) =>
      el(
        'div',
        { className: 'board-group' },
        el('h4', {}, title),
        el(
          'ul',
          {},
          ...list.map((i) => {
            const cb = el('input', { type: 'checkbox', checked: !!state.done[i.id], ariaLabel: `Mark ${i.title} complete` });
            cb.addEventListener('change', () => setDone(i.id, cb.checked));
            return el('li', {}, cb, el('a', { href: i.url }, i.title), i.time ? el('span', { className: 'time' }, i.time) : '');
          }),
        ),
      );
    node.append(group('Modules', data.modules), group('Projects', data.projects));
  }

  // ---- lesson pages
  function bindLesson() {
    const bar = document.querySelector('.lesson-bar[data-lesson]');
    if (bar) {
      state.last = { id: bar.dataset.lesson, title: bar.dataset.title, url: location.pathname, lang: bar.dataset.lesson.split('/')[0], at: Date.now() };
      state.lang = state.last.lang;
      save();
    }
    document.querySelectorAll('[data-done-toggle]').forEach((b) => {
      b.addEventListener('click', () => setDone(b.dataset.doneToggle, !state.done[b.dataset.doneToggle]));
    });
    const id = bar?.dataset.lesson;
    if (!id) return;
    // Learning-outcome checkboxes are rendered disabled by the markdown pipeline; make them interactive.
    const boxes = [...document.querySelectorAll('.sl-markdown-content input[type="checkbox"]')];
    const saved = state.outcomes[id] ?? [];
    boxes.forEach((cb, i) => {
      cb.disabled = false;
      cb.checked = !!saved[i];
      cb.addEventListener('change', () => {
        state.outcomes[id] = boxes.map((b) => b.checked);
        save();
      });
    });
  }

  function refreshButtons() {
    document.querySelectorAll('[data-done-toggle]').forEach((b) => {
      const done = !!state.done[b.dataset.doneToggle];
      b.classList.toggle('is-done', done);
      b.textContent = done ? `Completed (undo)` : `Mark ${b.dataset.label} complete`;
    });
  }

  function refreshSidebar() {
    document.querySelectorAll('.sidebar-content a[href]').forEach((a) => {
      const id = lessonIdFromPath(new URL(a.href).pathname);
      if (id) a.classList.toggle('lesson-done', !!state.done[id]);
    });
  }

  function refreshContinue() {
    document.querySelectorAll('[data-continue]').forEach((n) => {
      n.replaceChildren();
      if (!state.last) return;
      n.append(el('span', {}, 'Continue where you left off: '), el('a', { href: state.last.url }, state.last.title));
    });
  }

  function refresh() {
    document.querySelectorAll('.progress-board').forEach(renderBoard);
    refreshButtons();
    refreshSidebar();
  }

  // ---- language tabs (compare pages)
  function bindTabs() {
    const groups = [...document.querySelectorAll('.lang-tabs')];
    if (!groups.length) return;
    const show = (lang) => {
      groups.forEach((g) => {
        const available = [...g.querySelectorAll(':scope > .lang-panel')].map((p) => p.dataset.lang);
        const target = available.includes(lang) ? lang : available[0];
        g.querySelectorAll(':scope > .lang-panel').forEach((p) => p.classList.toggle('active', p.dataset.lang === target));
        g.querySelectorAll('.lang-tab-buttons button').forEach((b) => b.classList.toggle('active', b.dataset.lang === target));
      });
    };
    groups.forEach((g) =>
      g.querySelectorAll('.lang-tab-buttons button').forEach((b) =>
        b.addEventListener('click', () => {
          state.lang = b.dataset.lang;
          save();
          show(state.lang);
        }),
      ),
    );
    show(state.lang ?? 'go');
  }

  function init() {
    bindLesson();
    bindTabs();
    refreshContinue();
    refresh();
    document.querySelector('.reset-progress')?.addEventListener('click', () => {
      if (confirm('Reset all progress saved in this browser?')) {
        state.done = {};
        state.outcomes = {};
        delete state.last;
        save();
        location.reload();
      }
    });
  }

  window.addEventListener('storage', (e) => {
    if (e.key !== KEY) return;
    Object.assign(state, load());
    refresh();
  });
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init);
  else init();
})();
