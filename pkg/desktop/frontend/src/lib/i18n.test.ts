import { describe, expect, it } from 'vitest';
import { en, tr } from './i18n';

const placeholderPattern = /\{(\w+)\}/g;

function placeholders(value: string): string[] {
  return [...value.matchAll(placeholderPattern)].map((match) => match[1]).sort();
}

describe('localization catalogs', () => {
  it('has the same keys in English and Turkish', () => {
    expect(Object.keys(tr).sort()).toEqual(Object.keys(en).sort());
  });

  it('has no empty values', () => {
    for (const [lang, catalog] of Object.entries({ en, tr })) {
      for (const [key, value] of Object.entries(catalog)) {
        expect(value.trim(), `${lang}:${key}`).not.toBe('');
      }
    }
  });

  it('keeps interpolation placeholders in sync', () => {
    for (const key of Object.keys(en)) {
      expect(placeholders(tr[key]), key).toEqual(placeholders(en[key]));
    }
  });

  it('uses one Turkish term per concept', () => {
    expect(tr['skills.title']).toBe('Beceriler');
    expect(tr['settings.skills']).toBe('Beceriler');
    expect(tr['catalog.kind.skills']).toBe('Beceriler');
    expect(tr['ask.placeholder']).toBe('Yanıtınızı yazın…');
    expect(tr['common.fork']).toBe('Dallandır');
  });
});
