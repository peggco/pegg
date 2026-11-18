import { beforeEach, describe, expect, it } from 'vitest';
import {
  LANGUAGES,
  LANGUAGE_STORAGE,
  applyLanguage,
  en,
  normalizeLanguage,
  persistLanguage,
  readStoredLanguage,
  setLanguage,
  tr,
  translate,
} from './i18n';
import { store } from '../state/store';

const placeholderPattern = /\{(\w+)\}/g;

function placeholders(value: string): string[] {
  return [...value.matchAll(placeholderPattern)].map((match) => match[1]).sort();
}

beforeEach(() => {
  window.localStorage.clear();
  document.documentElement.lang = 'en';
  store.set({ language: 'en' });
});

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

describe('translate', () => {
  it('uses the Turkish catalog and falls back to the key', () => {
    expect(translate('tr', 'settings.language')).toBe('Dil');
    expect(translate('tr', 'settings.generalDesc')).toContain('Model');
    expect(translate('tr', 'missing.key')).toBe('missing.key');
  });

  it('lists English and Turkish', () => {
    expect(LANGUAGES.map((language) => language.code)).toEqual(['en', 'tr']);
  });
});

describe('language storage', () => {
  it('normalizes unsupported values to English', () => {
    expect(normalizeLanguage('tr')).toBe('tr');
    expect(normalizeLanguage('de')).toBe('en');
    expect(normalizeLanguage(undefined)).toBe('en');
  });

  it('reads and persists only supported languages', () => {
    persistLanguage('tr');
    expect(window.localStorage.getItem(LANGUAGE_STORAGE)).toBe('tr');
    expect(readStoredLanguage()).toBe('tr');

    window.localStorage.setItem(LANGUAGE_STORAGE, 'de');
    expect(readStoredLanguage()).toBeUndefined();
  });

  it('applies the language to the store and document', () => {
    expect(applyLanguage('tr')).toBe('tr');
    expect(store.get().language).toBe('tr');
    expect(document.documentElement.lang).toBe('tr');

    applyLanguage('de');
    expect(store.get().language).toBe('en');
  });

  it('caches the selected language', () => {
    expect(setLanguage('tr')).toBe('tr');
    expect(window.localStorage.getItem(LANGUAGE_STORAGE)).toBe('tr');
    expect(document.documentElement.lang).toBe('tr');
  });
});
