import type { AskQuestion } from '../types';

export type QuestionType = 'text' | 'select' | 'multiselect' | 'boolean';

export const MULTI_SEPARATOR = ', ';

export function questionType(question: AskQuestion): QuestionType {
  switch (question.type) {
    case 'select':
    case 'multiselect':
    case 'boolean':
      return question.type;
    default:
      return 'text';
  }
}

export function hasCustomOption(type: QuestionType): boolean {
  return type === 'select' || type === 'multiselect' || type === 'boolean';
}

export function isAnswered(value?: string): boolean {
  return Boolean(value && value.trim());
}

export function multiselectValues(value?: string): string[] {
  return (value ?? '').split(MULTI_SEPARATOR).filter(Boolean);
}

export function toggleMultiselect(current: string | undefined, option: string): string {
  const values = multiselectValues(current);
  const next = values.includes(option) ? values.filter((value) => value !== option) : [...values, option];
  return next.join(MULTI_SEPARATOR);
}

export function serializeAnswers(
  questions: AskQuestion[],
  answers: Record<string, string>,
): Record<string, string> {
  const result: Record<string, string> = {};
  for (const question of questions) {
    const value = (answers[question.id] ?? '').trim();
    if (value) result[question.id] = value;
  }
  return result;
}

export function firstMissingRequired(
  questions: AskQuestion[],
  answers: Record<string, string>,
): number {
  return questions.findIndex((question) => question.required && !isAnswered(answers[question.id]));
}

export function tabLabel(question: AskQuestion): string {
  const words = question.question.trim().split(/\s+/).filter(Boolean);
  const label = words.slice(0, 3).join(' ');
  if (!label) return question.id;
  return label.length > 20 ? `${label.slice(0, 19)}…` : label;
}
