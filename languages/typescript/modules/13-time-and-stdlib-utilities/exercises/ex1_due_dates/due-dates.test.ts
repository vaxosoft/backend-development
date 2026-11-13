import assert from 'node:assert/strict';
import { describe, test } from 'node:test';
import { daysUntilDue, isOverdue, parseDueDate } from './due-dates.ts';

describe('parseDueDate', () => {
  test('accepts a real calendar day', () => {
    const date = parseDueDate('2030-01-31');
    assert.ok(date);
    assert.equal(date.toString(), '2030-01-31');
    assert.equal(parseDueDate('2032-02-29')?.toString(), '2032-02-29');
  });

  test('rejects impossible days instead of clamping them', () => {
    assert.equal(parseDueDate('2030-02-30'), null);
    assert.equal(parseDueDate('2031-02-29'), null);
    assert.equal(parseDueDate('2030-13-01'), null);
  });

  test('rejects every other format', () => {
    for (const text of [
      '',
      ' 2030-01-31',
      '2030-1-5',
      '31/01/2030',
      '20300131',
      '2030-01-31T10:00:00',
    ]) {
      assert.equal(parseDueDate(text), null, JSON.stringify(text));
    }
  });
});

describe('daysUntilDue', () => {
  test('counts calendar days, not 24-hour blocks', () => {
    const now = Temporal.Instant.from('2030-01-30T23:59:00Z');
    assert.equal(daysUntilDue('2030-01-31', now, 'UTC'), 1);
    assert.equal(daysUntilDue('2030-01-30', now, 'UTC'), 0);
    assert.equal(daysUntilDue('2030-01-25', now, 'UTC'), -5);
  });

  test("uses today's date in the given time zone", () => {
    const now = Temporal.Instant.from('2030-01-31T23:30:00Z');
    assert.equal(daysUntilDue('2030-01-31', now, 'America/New_York'), 0);
    assert.equal(daysUntilDue('2030-01-31', now, 'Asia/Tokyo'), -1);
  });

  test('is correct across a daylight saving change', () => {
    const now = Temporal.Instant.from('2030-03-30T12:00:00Z');
    assert.equal(daysUntilDue('2030-04-01', now, 'Europe/Berlin'), 2);
  });
});

describe('isOverdue', () => {
  const now = Temporal.Instant.from('2030-02-01T12:00:00Z');

  test('past due date and not done', () => {
    assert.equal(isOverdue({ due_date: '2030-01-31', status: 'todo' }, now, 'UTC'), true);
    assert.equal(isOverdue({ due_date: '2030-01-31', status: 'in_progress' }, now, 'UTC'), true);
  });

  test('due today is not overdue yet', () => {
    assert.equal(isOverdue({ due_date: '2030-02-01', status: 'todo' }, now, 'UTC'), false);
  });

  test('done tasks and tasks without a due date are never overdue', () => {
    assert.equal(isOverdue({ due_date: '2030-01-01', status: 'done' }, now, 'UTC'), false);
    assert.equal(isOverdue({ due_date: null, status: 'todo' }, now, 'UTC'), false);
  });

  test('the zone decides which day it is', () => {
    const late = Temporal.Instant.from('2030-01-31T20:00:00Z');
    const task = { due_date: '2030-01-31', status: 'todo' } as const;
    assert.equal(isOverdue(task, late, 'Asia/Tokyo'), true);
    assert.equal(isOverdue(task, late, 'UTC'), false);
  });
});
