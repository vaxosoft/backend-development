import assert from 'node:assert/strict';
import { describe, test } from 'node:test';
import { nextDailyRun, upcomingRuns } from './next-run.ts';

const at = (iso: string) => Temporal.Instant.from(iso);
const next = (after: string, zone: string, hour: number, minute: number) =>
  nextDailyRun(at(after), zone, { hour, minute }).toString();

describe('nextDailyRun', () => {
  test('later today when the time has not passed yet', () => {
    assert.equal(next('2030-06-10T05:00:00Z', 'UTC', 9, 0), '2030-06-10T09:00:00Z');
  });

  test('tomorrow when the time has passed, and strictly after the reference instant', () => {
    assert.equal(next('2030-06-10T09:30:00Z', 'UTC', 9, 0), '2030-06-11T09:00:00Z');
    assert.equal(next('2030-06-10T09:00:00Z', 'UTC', 9, 0), '2030-06-11T09:00:00Z');
  });

  test('uses the wall clock of the zone', () => {
    assert.equal(next('2030-01-09T23:59:59Z', 'Asia/Tokyo', 9, 0), '2030-01-10T00:00:00Z');
    assert.equal(next('2030-01-10T00:00:00Z', 'Asia/Tokyo', 9, 0), '2030-01-11T00:00:00Z');
    assert.equal(next('2030-01-10T12:00:00Z', 'America/New_York', 9, 0), '2030-01-10T14:00:00Z');
  });

  test('keeps the wall-clock time across the spring-forward change (23-hour day)', () => {
    // Berlin: 09:00 is +01:00 on 03-30 and +02:00 on 03-31.
    assert.equal(next('2030-03-30T08:00:00Z', 'Europe/Berlin', 9, 0), '2030-03-31T07:00:00Z');
  });

  test('a time inside the spring-forward gap runs right after the gap', () => {
    // 02:30 does not exist on 2030-03-31 in Berlin; the clock jumps from 02:00 to 03:00.
    assert.equal(next('2030-03-30T01:30:00Z', 'Europe/Berlin', 2, 30), '2030-03-31T01:30:00Z');
    assert.equal(next('2030-03-31T01:30:00Z', 'Europe/Berlin', 2, 30), '2030-04-01T00:30:00Z');
  });

  test('a time in the fall-back overlap runs once, at its first occurrence', () => {
    // 02:30 happens at 00:30Z (+02:00) and again at 01:30Z (+01:00) on 2030-10-27.
    assert.equal(next('2030-10-26T22:00:00Z', 'Europe/Berlin', 2, 30), '2030-10-27T00:30:00Z');
    assert.equal(next('2030-10-27T00:30:00Z', 'Europe/Berlin', 2, 30), '2030-10-28T01:30:00Z');
    assert.equal(next('2030-10-27T01:00:00Z', 'Europe/Berlin', 2, 30), '2030-10-28T01:30:00Z');
  });
});

describe('upcomingRuns', () => {
  test('returns consecutive runs across a daylight saving change', () => {
    const runs = upcomingRuns(
      at('2030-03-29T12:00:00Z'),
      'Europe/Berlin',
      { hour: 8, minute: 0 },
      4,
    );
    assert.deepEqual(runs.map(String), [
      '2030-03-30T07:00:00Z',
      '2030-03-31T06:00:00Z',
      '2030-04-01T06:00:00Z',
      '2030-04-02T06:00:00Z',
    ]);
  });

  test('count 0 returns an empty list', () => {
    assert.deepEqual(
      upcomingRuns(at('2030-01-01T00:00:00Z'), 'UTC', { hour: 1, minute: 0 }, 0),
      [],
    );
  });
});
