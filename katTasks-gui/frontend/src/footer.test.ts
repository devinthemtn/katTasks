import { test } from 'node:test';
import assert from 'node:assert/strict';
import { pieces } from './footer.ts';

const sep = { text: ' • ', style: 'dim' };

test('a key and its label become one button', () => {
  assert.deepEqual(pieces([{ text: 'a', style: 'key', action: 'add' }, { text: ' add', style: 'dim', action: 'add' }]),
    [{ action: 'add', key: 'a', label: ' add' }]);
});

test('the separator between buttons is dropped', () => {
  const got = pieces([
    { text: 'a', style: 'key', action: 'add' }, { text: ' add', style: 'dim', action: 'add' }, sep,
    { text: 'e', style: 'key', action: 'edit' }, { text: ' edit', style: 'dim', action: 'edit' },
  ]);
  assert.deepEqual(got, [{ action: 'add', key: 'a', label: ' add' }, { action: 'edit', key: 'e', label: ' edit' }]);
});

test('a line without actions stays text', () => {
  const line = [{ text: 'saved', style: 'ok' }, sep, { text: 'more', style: 'dim' }];
  assert.deepEqual(pieces(line), [{ segs: line }]);
});

test('a pending key chip stays in front of the buttons', () => {
  const chip = { text: 'g…', style: 'chip' };
  const got = pieces([chip, { text: ' ' }, { text: 'x', style: 'key', action: 'toggle_done' }, { text: ' done', style: 'dim', action: 'toggle_done' }]);
  assert.deepEqual(got, [{ segs: [chip, { text: ' ' }] }, { action: 'toggle_done', key: 'x', label: ' done' }]);
});
