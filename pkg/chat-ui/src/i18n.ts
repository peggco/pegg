export type Language = 'en' | 'tr';

const en: Record<string, string> = {
  'revert.title.one': 'Reverted {count} file',
  'revert.title.other': 'Reverted {count} files',
  'revert.title.none': 'Reverted changes',
  'revert.undo': 'Undo',
  'revert.undoing': 'Undoing…',
  'revert.restored': 'Changes restored',
  'revert.restoredDetail': 'The reverted messages are back.',
  'revert.showFiles': 'Show reverted files',
  'revert.hideFiles': 'Hide reverted files',
  'revert.dismiss': 'Dismiss',
  'revert.undoFailed': 'Could not undo the revert.',
  'revert.status.reverted': 'Reverted',
  'revert.status.conflict': 'Conflict',
  'revert.status.skipped': 'Skipped',
  'revert.conflictHint': 'This file was modified locally, so it was left untouched.',
  'revert.skippedHint': 'This file could not be restored.',
};

const tr: Record<string, string> = {
  'revert.title.one': '{count} dosya geri alındı',
  'revert.title.other': '{count} dosya geri alındı',
  'revert.title.none': 'Değişiklikler geri alındı',
  'revert.undo': 'Geri al',
  'revert.undoing': 'Geri alınıyor…',
  'revert.restored': 'Değişiklikler geri yüklendi',
  'revert.restoredDetail': 'Geri alınan mesajlar geri geldi.',
  'revert.showFiles': 'Geri alınan dosyaları göster',
  'revert.hideFiles': 'Geri alınan dosyaları gizle',
  'revert.dismiss': 'Kapat',
  'revert.undoFailed': 'Geri alma işlemi geri alınamadı.',
  'revert.status.reverted': 'Geri alındı',
  'revert.status.conflict': 'Çakışma',
  'revert.status.skipped': 'Atlandı',
  'revert.conflictHint': 'Bu dosya yerel olarak değiştirildiği için dokunulmadı.',
  'revert.skippedHint': 'Bu dosya geri yüklenemedi.',
};

const catalogs: Record<Language, Record<string, string>> = { en, tr };

export function resolveLanguage(language?: string | null): Language {
  const code = (language ?? '').toLowerCase().split(/[-_]/)[0];
  return code === 'tr' ? 'tr' : 'en';
}

export function translate(
  language: string | null | undefined,
  key: string,
  vars?: Record<string, string | number>,
): string {
  const code = resolveLanguage(language);
  let value = catalogs[code][key] ?? en[key] ?? key;
  if (vars) {
    for (const [name, replacement] of Object.entries(vars)) {
      value = value.replaceAll(`{${name}}`, String(replacement));
    }
  }
  return value;
}
