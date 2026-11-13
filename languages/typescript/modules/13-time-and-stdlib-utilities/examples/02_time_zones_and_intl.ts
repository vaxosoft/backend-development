// Time zones: store Instants (UTC), convert to a zone only to display or to do calendar math.
const created = Temporal.Instant.from('2030-03-30T22:30:00Z');

// 1. One instant, many wall clocks.
for (const zone of ['UTC', 'Europe/Berlin', 'Asia/Tokyo', 'America/New_York']) {
  console.log(zone.padEnd(17), created.toZonedDateTimeISO(zone).toString());
}

// 2. Calendar math happens on ZonedDateTime, so a "day" can be 23 or 25 hours.
const berlinMidnight = Temporal.ZonedDateTime.from('2030-03-31T00:00:00[Europe/Berlin]');
const nextDay = berlinMidnight.add({ days: 1 });
const plus24h = berlinMidnight.add({ hours: 24 });
console.log('+1 day :', nextDay.toString());
console.log('+24 h  :', plus24h.toString());

// 3. The day Berlin springs forward is 23 hours long.
const elapsedMs = nextDay.epochMilliseconds - berlinMidnight.epochMilliseconds;
console.log('hours elapsed between the two midnights:', elapsedMs / 3_600_000);
console.log('hoursInDay on 2030-03-31:', berlinMidnight.hoursInDay);

// 4. Local times that do not exist (gap) or exist twice (overlap).
const gap = '2030-03-31T02:30:00[Europe/Berlin]';
console.log('gap, compatible :', Temporal.ZonedDateTime.from(gap).toString());
console.log(
  'gap, earlier    :',
  Temporal.ZonedDateTime.from(gap, { disambiguation: 'earlier' }).toString(),
);
try {
  Temporal.ZonedDateTime.from(gap, { disambiguation: 'reject' });
} catch (error) {
  console.log('gap, reject     :', error instanceof RangeError ? 'RangeError' : 'other');
}
const overlap = '2030-10-27T02:30:00[Europe/Berlin]';
console.log(
  'overlap, earlier:',
  Temporal.ZonedDateTime.from(overlap, { disambiguation: 'earlier' }).toString(),
);
console.log(
  'overlap, later  :',
  Temporal.ZonedDateTime.from(overlap, { disambiguation: 'later' }).toString(),
);

// 5. Intl.DateTimeFormat formats an instant for a human; always pass locale and timeZone explicitly.
const date = new Date(created.epochMilliseconds);
const format = (locale: string, timeZone: string) =>
  new Intl.DateTimeFormat(locale, { dateStyle: 'full', timeStyle: 'short', timeZone }).format(date);
console.log(format('en-US', 'America/New_York'));
console.log(format('de-DE', 'Europe/Berlin'));
console.log(format('en-GB', 'Asia/Tokyo'));

// 6. Temporal objects format themselves with toLocaleString.
console.log(
  created
    .toZonedDateTimeISO('Asia/Tokyo')
    .toLocaleString('en-US', { dateStyle: 'medium', timeStyle: 'short' }),
);

// 7. Machine formats: toString() is ISO 8601 / RFC 3339, the right choice for JSON.
console.log(JSON.stringify({ created_at: created.toString() }));
const withOffset = Temporal.Instant.from('2030-03-31T00:30:00+02:00');
console.log('offset input normalised to UTC:', withOffset.toString());
