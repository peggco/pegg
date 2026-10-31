import type { Theme } from '../types';

export const THEME_STORAGE = 'pegg.theme';

export function prefersDark(): boolean {
  return typeof window !== 'undefined' && window.matchMedia('(prefers-color-scheme: dark)').matches;
}

function siblingName(themes: Theme[], name: string, dark: boolean): string | undefined {
  const pairs: Array<[string, string]> = [
    ['light', 'dark'],
    ['catppuccin-latte', 'catppuccin-mocha'],
    ['tokyonight-day', 'tokyonight-night'],
    ['gruvbox-light', 'gruvbox-dark'],
    ['solarized-light', 'solarized-dark'],
    ['rosepine-dawn', 'rosepine'],
    ['everforest-light', 'everforest-dark'],
  ];
  for (const [light, darkName] of pairs) {
    if (name === light && dark) return darkName;
    if (name === darkName && !dark) return light;
  }
  const replaced = name.endsWith('-light')
    ? name.slice(0, -6) + '-dark'
    : name.endsWith('-dark')
      ? name.slice(0, -5) + '-light'
      : '';
  if (replaced && themes.some((t) => t.name === replaced)) return replaced;
  return undefined;
}

export function resolveTheme(themes: Theme[], requested: string, mode: 'auto' | 'light' | 'dark'): Theme {
  const find = (n: string) => themes.find((t) => t.name === n);
  const aliases: Record<string, string> = { 'pegg-light': 'light', 'pegg-dark': 'dark' };
  const name = aliases[requested] ?? requested;
  if (mode === 'auto') {
    const base = find(name) ?? find('dark') ?? themes[0];
    if (!base) return fallbackTheme();
    const dark = prefersDark();
    if (base.dark === dark) return base;
    const sib = siblingName(themes, base.name, dark);
    if (sib) return find(sib) ?? base;
    return (dark ? find('dark') : find('light')) ?? base;
  }
  const base = find(name) ?? find('dark') ?? themes[0];
  if (!base) return fallbackTheme();
  if (base.dark !== (mode === 'dark')) {
    const sib = siblingName(themes, base.name, mode === 'dark');
    if (sib) return find(sib) ?? base;
  }
  return base;
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

export function fallbackTheme(): Theme {
  return {
    name: 'dark',
    label: 'Dark',
    dark: true,
    tokens: {
      '--pg-bg': '#18181b',
      '--pg-sidebar': '#202023',
      '--pg-surface': '#27272a',
      '--pg-text': '#fafafa',
      '--pg-accent': '#3b82f6',
      '--pg-border': '#3f3f46',
    },
  };
}
