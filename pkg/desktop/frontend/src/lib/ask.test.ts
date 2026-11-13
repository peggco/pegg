import { describe, expect, it } from 'vitest';
import type { AskQuestion } from '../types';
import {
  firstMissingRequired,
  hasCustomOption,
  isAnswered,
  multiselectValues,
  questionType,
  serializeAnswers,
  tabLabel,
  toggleMultiselect,
} from './ask';

function question(overrides: Partial<AskQuestion> & { id: string }): AskQuestion {
  return {
    id: overrides.id,
    question: overrides.question ?? `Question ${overrides.id}?`,
    type: overrides.type ?? 'text',
    options: overrides.options,
    required: overrides.required,
  };
}

describe('questionType', () => {
  it('keeps known types and falls back to text', () => {
    expect(questionType(question({ id: 'a', type: 'select' }))).toBe('select');
    expect(questionType(question({ id: 'b', type: 'multiselect' }))).toBe('multiselect');
    expect(questionType(question({ id: 'c', type: 'boolean' }))).toBe('boolean');
    expect(questionType(question({ id: 'd', type: 'other' }))).toBe('text');
    expect(questionType(question({ id: 'e' }))).toBe('text');
  });
});

describe('hasCustomOption', () => {
  it('only offers custom answers for choice and boolean questions', () => {
    expect(hasCustomOption('select')).toBe(true);
    expect(hasCustomOption('multiselect')).toBe(true);
    expect(hasCustomOption('boolean')).toBe(true);
    expect(hasCustomOption('text')).toBe(false);
  });
});

describe('isAnswered', () => {
  it('treats blank values as missing', () => {
    expect(isAnswered(undefined)).toBe(false);
    expect(isAnswered('')).toBe(false);
    expect(isAnswered('   ')).toBe(false);
    expect(isAnswered('Pegg')).toBe(true);
  });
});

describe('multiselectHelpers', () => {
  it('splits and joins with the protocol separator', () => {
    expect(multiselectValues(undefined)).toEqual([]);
    expect(multiselectValues('')).toEqual([]);
    expect(multiselectValues('a, b')).toEqual(['a', 'b']);
  });

  it('toggles options', () => {
    expect(toggleMultiselect('', 'a')).toBe('a');
    expect(toggleMultiselect('a', 'b')).toBe('a, b');
    expect(toggleMultiselect('a, b', 'a')).toBe('b');
  });
});

describe('serializeAnswers', () => {
  it('trims values and omits empty answers', () => {
    const questions = [
      question({ id: 'a' }),
      question({ id: 'b' }),
      question({ id: 'c' }),
    ];
    expect(serializeAnswers(questions, { a: '  yes ', b: '', c: '   ' })).toEqual({ a: 'yes' });
  });

  it('keeps multiselect joined values', () => {
    const questions = [question({ id: 'a', type: 'multiselect', options: ['x', 'y'] })];
    expect(serializeAnswers(questions, { a: 'x, y' })).toEqual({ a: 'x, y' });
  });
});

describe('firstMissingRequired', () => {
  it('ignores optional questions and reports the first required gap', () => {
    const questions = [
      question({ id: 'a', required: false }),
      question({ id: 'b', required: true }),
      question({ id: 'c', required: true }),
    ];
    expect(firstMissingRequired(questions, { b: 'ok', c: 'ok' })).toBe(-1);
    expect(firstMissingRequired(questions, { c: 'ok' })).toBe(1);
    expect(firstMissingRequired(questions, {})).toBe(1);
  });
});

describe('tabLabel', () => {
  it('uses at most three words and truncates long labels', () => {
    expect(tabLabel(question({ id: 'a', question: 'Pick a deployment target' }))).toBe('Pick a deployment');
    expect(tabLabel(question({ id: 'b', question: 'abcdefghij klmnopqrst uvwxyz' }))).toBe('abcdefghij klmnopqr…');
    expect(tabLabel(question({ id: 'c', question: '   ' }))).toBe('c');
  });
});
