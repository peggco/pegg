import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { PluginCard, ViewLines, ViewStatusBar } from './ViewChrome';

describe('ViewChrome', () => {
  it('renders styled spans', () => {
    render(<ViewLines lines={[{ spans: [{ text: 'warn', style: 'warning' }] }]} />);
    expect(screen.getByText('warn').className).toContain('view-warning');
  });

  it('renders both status sides', () => {
    render(<ViewStatusBar view={{ statusLeft: [{ text: 'L' }], statusRight: [{ text: 'R' }] }} />);
    expect(screen.getByText('L')).toBeTruthy();
    expect(screen.getByText('R')).toBeTruthy();
  });

  it('renders a plugin card', () => {
    render(
      <PluginCard fragment={{ id: 'x', title: 'CI', badge: 'passing', lines: [{ text: 'ok' }] }} />,
    );
    expect(screen.getByText('CI')).toBeTruthy();
    expect(screen.getByText('passing')).toBeTruthy();
    expect(screen.getByText('ok')).toBeTruthy();
  });
});
