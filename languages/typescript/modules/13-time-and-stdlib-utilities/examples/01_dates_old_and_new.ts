// Legacy Date pitfalls next to their Temporal replacements.
process.env.TZ = 'America/New_York'; // pin the local zone so the output is the same everywhere

// 1. Date months are 0-based, Temporal months are 1-based.
const legacy = new Date(2030, 0, 31);
console.log('Date month for January:', legacy.getMonth());
console.log('Temporal month for January:', Temporal.PlainDate.from('2030-01-31').month);

// 2. Date overflows silently, Temporal clamps (or throws when asked to).
legacy.setMonth(1);
console.log('Date Jan 31 + 1 month:', `${legacy.getMonth() + 1}-${legacy.getDate()}`);
const jan31 = Temporal.PlainDate.from('2030-01-31');
console.log('Temporal Jan 31 + 1 month:', jan31.add({ months: 1 }).toString());
try {
  jan31.add({ months: 1 }, { overflow: 'reject' });
} catch (error) {
  console.log('overflow: reject ->', error instanceof RangeError ? 'RangeError' : 'other');
}

// 3. A date-only string is UTC, a date-time string without offset is local.
console.log('Date("2030-01-01"):      ', new Date('2030-01-01').toISOString());
console.log('Date("2030-01-01T00:00"):', new Date('2030-01-01T00:00').toISOString());

// 4. Temporal separates the concepts: Instant (a point), PlainDate (a calendar day), Duration.
const created = Temporal.Instant.from('2030-01-01T09:00:00Z');
console.log('Instant:', created.toString(), created.epochMilliseconds);
const due = Temporal.PlainDate.from('2030-01-31');
console.log('PlainDate:', due.toString(), due.dayOfWeek, due.daysInMonth);

// 5. Arithmetic and comparison.
const sprint = Temporal.Duration.from({ days: 14 });
console.log('Due + 14 days:', due.add(sprint).toString());
console.log('Days between:', Temporal.PlainDate.from('2030-01-01').until(due).days);
console.log('Compare:', Temporal.PlainDate.compare(due, jan31), due.equals(jan31));
const long = Temporal.Duration.from({ hours: 90, minutes: 30 });
console.log('Total hours:', long.total('hours'));
console.log('Balanced:', long.round({ largestUnit: 'day' }).toString());

// 6. Invalid input throws instead of producing "Invalid Date".
console.log('Invalid Date:', String(new Date('2030-02-30x')));
try {
  Temporal.PlainDate.from('2030-02-30', { overflow: 'reject' });
} catch (error) {
  console.log('Temporal rejects 2030-02-30:', error instanceof RangeError);
}

// 7. Interop at the edges: Date <-> Instant.
console.log('Date -> Instant:', new Date(0).toTemporalInstant().toString());
console.log('Instant -> Date:', new Date(created.epochMilliseconds).toISOString());
