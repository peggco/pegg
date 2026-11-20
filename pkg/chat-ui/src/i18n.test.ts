import { describe, expect, it } from 'vitest';
import { resolveLanguage, translate } from './i18n';

describe('chat-ui i18n', () => {
  it('resolves supported languages and defaults to English', () => {
    expect(resolveLanguage('tr')).toBe('tr');
    expect(resolveLanguage('tr-TR')).toBe('tr');
    expect(resolveLanguage('de')).toBe('en');
    expect(resolveLanguage(undefined)).toBe('en');
  });

  it('pluralizes the revert title', () => {
    expect(translate('en', 'revert.title.one', { count: 1 })).toBe('Reverted 1 file');
    expect(translate('en', 'revert.title.other', { count: 3 })).toBe('Reverted 3 files');
    expect(translate('tr', 'revert.title.other', { count: 3 })).toBe('3 dosya geri alındı');
  });

  it('localizes the undo action and conflicts', () => {
    expect(translate('en', 'revert.undo')).toBe('Undo');
    expect(translate('tr', 'revert.undo')).toBe('Geri al');
    expect(translate('tr', 'revert.status.conflict')).toBe('Çakışma');
  });
});
