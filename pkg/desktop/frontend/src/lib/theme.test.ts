import { describe, expect, it, vi } from 'vitest';
import { resolveTheme } from './theme';
import type { Theme } from '../types';

const themes: Theme[] = [
  { name: 'dark', label: 'Dark', dark: true, tokens: { '--pg-bg': '#18181b', '--pg-accent': '#3b82f6' } },
  { name: 'light', label: 'Light', dark: false, tokens: { '--pg-bg': '#ffffff', '--pg-accent': '#18181b' } },
  { name: 'catppuccin-latte', label: 'Latte', dark: false, tokens: { '--pg-bg': '#eff1f5' } },
  { name: 'catppuccin-mocha', label: 'Mocha', dark: true, tokens: { '--pg-bg': '#1e1e2e' } },
];

function mockPrefers(dark: boolean) {
  vi.stubGlobal('matchMedia', () => ({ matches: dark, addEventListener: () => undefined, removeEventListener: () => undefined }));
}

describe('theme resolution', () => {
  it('migrates legacy pegg themes', () => {
    mockPrefers(true);
    expect(resolveTheme(themes, 'pegg-light', 'auto').name).toBe('dark');
  });

  it('follows system mode', () => {
    mockPrefers(true);
    expect(resolveTheme(themes, 'light', 'auto').name).toBe('dark');
    mockPrefers(false);
    expect(resolveTheme(themes, 'dark', 'auto').name).toBe('light');
  });

  it('honours explicit light/dark mode', () => {
    mockPrefers(true);
    expect(resolveTheme(themes, 'dark', 'light').name).toBe('light');
    expect(resolveTheme(themes, 'light', 'dark').name).toBe('dark');
  });
});
