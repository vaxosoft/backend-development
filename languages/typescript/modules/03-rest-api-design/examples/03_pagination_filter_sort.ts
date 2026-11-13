// Pagination, filtering and sorting driven by query-string parameters.
import { HttpError, listTasks, parseListQuery, type Task } from './_shared.ts';

const tasks: Task[] = [
  {
    id: 1,
    title: 'Write docs',
    status: 'todo',
    due_date: '2030-03-01',
    tags: ['docs'],
    owner_id: null,
    created_at: '2030-01-01T09:00:00Z',
  },
  {
    id: 2,
    title: 'Ship it',
    status: 'in_progress',
    due_date: null,
    tags: ['release'],
    owner_id: 1,
    created_at: '2030-01-02T09:00:00Z',
  },
  {
    id: 3,
    title: 'Review PR',
    status: 'todo',
    due_date: '2030-02-01',
    tags: ['docs', 'review'],
    owner_id: 2,
    created_at: '2030-01-03T09:00:00Z',
  },
  {
    id: 4,
    title: 'Plan sprint',
    status: 'done',
    due_date: '2030-01-15',
    tags: [],
    owner_id: null,
    created_at: '2030-01-04T09:00:00Z',
  },
  {
    id: 5,
    title: 'Fix bug',
    status: 'todo',
    due_date: null,
    tags: ['bug'],
    owner_id: 1,
    created_at: '2030-01-05T09:00:00Z',
  },
];

function show(label: string, query: Record<string, string | string[]>): void {
  try {
    const page = listTasks(tasks, parseListQuery(query));
    const ids = page.items.map((task) => task.id);
    console.log(
      label.padEnd(28),
      `ids=[${ids}] total=${page.total} limit=${page.limit} offset=${page.offset}`,
    );
  } catch (error) {
    if (!(error instanceof HttpError)) throw error;
    console.log(label.padEnd(28), `${error.statusCode} ${error.message}`);
  }
}

show('defaults', {});
show('status=todo', { status: 'todo' });
show('tag=docs', { tag: 'docs' });
show('sort=due_date', { sort: 'due_date' });
show('sort=-due_date', { sort: '-due_date' });
show('limit=2&offset=2', { limit: '2', offset: '2' });
show('limit=1000 (clamped)', { limit: '1000' });
show('offset past the end', { offset: '50' });

// Query values arrive as strings: validate them, do not just call Number().
show('limit=abc', { limit: 'abc' });
show('limit=0', { limit: '0' });
show('limit=-1', { limit: '-1' });
show('limit=1&limit=2 (repeated)', { limit: ['1', '2'] });
show('sort=title (not allowed)', { sort: 'title' });
console.log(
  Number(undefined),
  Number(''),
  Number('abc'),
  '<- what Number() makes of missing values',
);
