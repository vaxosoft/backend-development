// Task Tracker: due dates, an injectable clock and an in-process background sweep.
import { setInterval as every } from 'node:timers/promises';

interface Clock {
  now(): Temporal.Instant;
}

type TaskStatus = 'todo' | 'in_progress' | 'done';

interface Task {
  id: number;
  title: string;
  status: TaskStatus;
  due_date: string | null;
  tags: string[];
  owner_id: number | null;
  created_at: string;
}

const TIME_ZONE = 'Europe/Berlin';

function createTask(id: number, title: string, dueDate: string | null, clock: Clock): Task {
  if (dueDate !== null) Temporal.PlainDate.from(dueDate, { overflow: 'reject' });
  return {
    id,
    title,
    status: 'todo',
    due_date: dueDate,
    tags: [],
    owner_id: null,
    created_at: clock.now().toString({ smallestUnit: 'second' }),
  };
}

type DueState = 'no_due_date' | 'overdue' | 'due_today' | 'upcoming';

// "Today" depends on the zone of the user, not on UTC.
function dueState(task: Task, now: Temporal.Instant): DueState {
  if (task.due_date === null || task.status === 'done') return 'no_due_date';
  const today = now.toZonedDateTimeISO(TIME_ZONE).toPlainDate();
  const order = Temporal.PlainDate.compare(Temporal.PlainDate.from(task.due_date), today);
  return order < 0 ? 'overdue' : order === 0 ? 'due_today' : 'upcoming';
}

function byDueDate(a: Task, b: Task): number {
  if (a.due_date === b.due_date) return a.id - b.id;
  if (a.due_date === null) return 1;
  if (b.due_date === null) return -1;
  return Temporal.PlainDate.compare(a.due_date, b.due_date);
}

// A clock that moves one day on every reading, so the sweep below is deterministic.
function steppingClock(start: string): Clock {
  let current = Temporal.Instant.from(start);
  return {
    now() {
      const reading = current;
      current = current.add({ hours: 24 });
      return reading;
    },
  };
}

const creation: Clock = { now: () => Temporal.Instant.from('2030-01-01T09:00:00Z') };
const tasks = [
  createTask(1, 'Write docs', '2030-01-31', creation),
  createTask(2, 'Ship release', '2030-02-02', creation),
  createTask(3, 'Someday', null, creation),
  createTask(4, 'Fix bug', '2030-01-30', creation),
];
console.log(JSON.stringify(tasks[0]));
console.log(
  'sorted:',
  [...tasks]
    .sort(byDueDate)
    .map((t) => `${t.id}:${t.due_date}`)
    .join(' '),
);

try {
  createTask(5, 'Bad date', '2030-02-30', creation);
} catch (error) {
  console.log('rejected due date:', error instanceof RangeError);
}

// Background sweep: every tick looks at "now" and reports what became overdue.
const clock = steppingClock('2030-01-30T10:00:00Z');
const reported = new Set<number>();
let tick = 0;
for await (const _ of every(5)) {
  const now = clock.now();
  const newlyOverdue = tasks.filter((t) => dueState(t, now) === 'overdue' && !reported.has(t.id));
  for (const task of newlyOverdue) reported.add(task.id);
  const local = now.toZonedDateTimeISO(TIME_ZONE).toPlainDate().toString();
  const dueToday = tasks.filter((t) => dueState(t, now) === 'due_today').map((t) => t.id);
  const overdue = newlyOverdue.map((t) => t.id);
  console.log(`tick ${tick} (${local}): newly overdue [${overdue}] due today [${dueToday}]`);
  tick += 1;
  if (tick === 4) break;
}
