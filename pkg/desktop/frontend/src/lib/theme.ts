import type { Theme } from '../types';

export const THEME_STORAGE = 'pegg.theme';

export function prefersDark(): boolean {
  return typeof window !== 'undefined' && window.matchMedia('(prefers-color-scheme: dark)').matches;
}

export function resolveTheme(themes: Theme[], requested: string, mode: 'auto' | 'light' | 'dark'): Theme {
  const find = (name: string) => themes.find((theme) => theme.name === name);
  const name = requested === 'pegg-light' ? 'light' : requested === 'pegg-dark' ? 'dark' : requested;
  const target = mode === 'auto' ? (prefersDark() ? 'dark' : 'light') : mode;
  return (
    find(target) ??
    find(name) ??
    themes.find((theme) => theme.dark === (target === 'dark')) ??
    fallbackTheme(target === 'dark')
  );
}

export function applyTheme(theme: Theme, fontScale = 1): void {
  const root = document.documentElement;
  root.dataset.theme = theme.name;
  root.dataset.dark = theme.dark ? 'true' : 'false';
  root.style.colorScheme = theme.dark ? 'dark' : 'light';
  for (const [key, value] of Object.entries(theme.tokens)) {
    if (key.startsWith('--')) root.style.setProperty(key, value);
  }
  root.style.setProperty('--pg-font-scale', String(fontScale));
  root.style.background = theme.tokens['--pg-bg'] ?? '';
  try {
    localStorage.setItem(
      THEME_STORAGE,
      JSON.stringify({ name: theme.name, dark: theme.dark, tokens: { '--pg-bg': theme.tokens['--pg-bg'] } }),
    );
  } catch {
    /* storage may be unavailable */
  }
}

export function fallbackTheme(dark = true): Theme {
  if (!dark) {
    return {
      name: 'light',
      label: 'Light',
      dark: false,
      tokens: {
        '--pg-bg': '#ffffff',
        '--pg-sidebar': '#ffffff',
        '--pg-surface': '#fafafa',
        '--pg-surface-2': '#f5f5f5',
        '--pg-text': '#0d0d0d',
        '--pg-text-2': '#404040',
        '--pg-muted': '#737373',
        '--pg-accent': '#171717',
        '--pg-accent-fg': '#ffffff',
        '--pg-border': '#e5e5e5',
        '--pg-border-strong': '#d4d4d4',
        '--pg-hover': '#f5f5f5',
        '--pg-active': '#ededed',
        '--pg-input-bg': '#ffffff',
        '--pg-input-text': '#0d0d0d',
        '--pg-placeholder': '#737373',
        '--pg-selection': '#d4d4d4',
        '--pg-code-bg': '#fafafa',
        '--pg-code-border': '#e5e5e5',
        '--pg-code-text': '#0d0d0d',
        '--pg-user-bg': '#f5f5f5',
        '--pg-shadow': 'rgba(0,0,0,0.08)',
      },
    };
  }
  return {
    name: 'dark',
    label: 'Dark',
    dark: true,
    tokens: {
      '--pg-bg': '#0d0d0d',
      '--pg-sidebar': '#0d0d0d',
      '--pg-surface': '#141414',
      '--pg-surface-2': '#1a1a1a',
      '--pg-text': '#fafafa',
      '--pg-text-2': '#d4d4d4',
      '--pg-muted': '#a3a3a3',
      '--pg-accent': '#fafafa',
      '--pg-accent-fg': '#0a0a0a',
      '--pg-border': '#262626',
      '--pg-border-strong': '#3f3f3f',
      '--pg-hover': '#1a1a1a',
      '--pg-active': '#222222',
      '--pg-input-bg': '#141414',
      '--pg-input-text': '#fafafa',
      '--pg-placeholder': '#a3a3a3',
      '--pg-selection': '#404040',
      '--pg-code-bg': '#141414',
      '--pg-code-border': '#262626',
      '--pg-code-text': '#e5e5e5',
      '--pg-user-bg': '#1a1a1a',
      '--pg-shadow': 'rgba(0,0,0,0.42)',
    },
  };
}
