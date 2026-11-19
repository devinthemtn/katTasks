import { test } from 'node:test';
import assert from 'node:assert/strict';
import { teaKey, isPaste, zoomStep, type KeyLike } from './keys.ts';

const k = (key: string, mods: Partial<KeyLike> = {}): KeyLike =>
  ({ key, ctrlKey: false, altKey: false, shiftKey: false, metaKey: false, ...mods });

test('plain characters pass through', () => {
  assert.equal(teaKey(k('j')), 'j');
  assert.equal(teaKey(k('G', { shiftKey: true })), 'G');
  assert.equal(teaKey(k(' ')), ' ');
  assert.equal(teaKey(k(':', { shiftKey: true })), ':');
  assert.equal(teaKey(k('é')), 'é');
});

test('named keys and modifiers', () => {
  assert.equal(teaKey(k('Enter')), 'enter');
  assert.equal(teaKey(k('Escape')), 'esc');
  assert.equal(teaKey(k('Tab', { shiftKey: true })), 'shift+tab');
  assert.equal(teaKey(k('ArrowDown')), 'down');
  assert.equal(teaKey(k('PageDown')), 'pgdown');
  assert.equal(teaKey(k('ArrowLeft', { ctrlKey: true })), 'ctrl+left');
  assert.equal(teaKey(k('ArrowLeft', { altKey: true })), 'alt+left');
  assert.equal(teaKey(k('Backspace', { ctrlKey: true })), 'alt+backspace');
  assert.equal(teaKey(k('d', { ctrlKey: true })), 'ctrl+d');
  assert.equal(teaKey(k('R', { ctrlKey: true, shiftKey: true })), 'ctrl+r');
  assert.equal(teaKey(k('b', { altKey: true })), 'alt+b');
  assert.equal(teaKey(k('F5')), 'f5');
});

test('ignored keys', () => {
  assert.equal(teaKey(k('Shift', { shiftKey: true })), null);
  assert.equal(teaKey(k('Dead')), null);
  assert.equal(teaKey(k('c', { metaKey: true })), null);
  assert.equal(teaKey(k('a', { isComposing: true })), null);
  assert.equal(teaKey(k('1', { ctrlKey: true })), null);
});

test('AltGr types its character', () => {
  assert.equal(teaKey(k('@', { ctrlKey: true, altKey: true })), '@');
});

test('paste and zoom shortcuts', () => {
  assert.ok(isPaste(k('v', { ctrlKey: true })));
  assert.ok(isPaste(k('V', { ctrlKey: true, shiftKey: true })));
  assert.ok(isPaste(k('Insert', { shiftKey: true })));
  assert.ok(!isPaste(k('v')));
  assert.equal(zoomStep(k('=', { ctrlKey: true })), 1);
  assert.equal(zoomStep(k('-', { ctrlKey: true })), -1);
  assert.equal(zoomStep(k('0', { ctrlKey: true })), 0);
  assert.equal(zoomStep(k('=')), null);
});
