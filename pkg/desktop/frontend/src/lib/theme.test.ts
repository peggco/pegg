import { describe, expect, it, vi } from 'vitest';
import { resolveTheme } from './theme';
import type { Theme } from '../types';

const themes: Theme[] = [
  { name: 'dark', label: 'Dark', dark: true, tokens: { '--pg-bg': '#0d0d0d', '--pg-accent': '#fafafa' } },
  { name: 'light', label: 'Light', dark: false, tokens: { '--pg-bg': '#ffffff', '--pg-accent': '#171717' } },
];

function mockPrefers(dark: boolean) {
  vi.stubGlobal('matchMedia', () => ({
    matches: dark,
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
  }));
}

describe('theme resolution', () => {
  it('migrates legacy pegg theme names', () => {
    mockPrefers(false);
    expect(resolveTheme(themes, 'pegg-light', 'light').name).toBe('light');
    expect(resolveTheme(themes, 'pegg-dark', 'dark').name).toBe('dark');
  });

  it('follows the system in auto mode', () => {
    mockPrefers(true);
    expect(resolveTheme(themes, 'light', 'auto').name).toBe('dark');
    mockPrefers(false);
    expect(resolveTheme(themes, 'dark', 'auto').name).toBe('light');
  });

  it('honours an explicit light/dark mode', () => {
    mockPrefers(true);
    expect(resolveTheme(themes, 'dark', 'light').name).toBe('light');
    expect(resolveTheme(themes, 'light', 'dark').name).toBe('dark');
  });

  it('falls back to light or dark when no themes are provided', () => {
    expect(resolveTheme([], 'light', 'light')).toMatchObject({ name: 'light', dark: false });
    expect(resolveTheme([], 'dark', 'dark')).toMatchObject({ name: 'dark', dark: true });
  });
});
