import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { store } from '../state/store';
import { ClosingOverlay } from './ClosingOverlay';

describe('ClosingOverlay', () => {
  beforeEach(() => {
    store.set({ language: 'en' });
  });

  afterEach(() => {
    cleanup();
  });

  it('renders nothing while the app is running', () => {
    const { container } = render(<ClosingOverlay active={false} />);
    expect(container.firstChild).toBeNull();
  });

  it('shows the localized closing status with a spinner', () => {
    const { container } = render(<ClosingOverlay active />);
    expect(screen.getByRole('status')).toBeTruthy();
    expect(screen.getByText('Closing Pegg…')).toBeTruthy();
    expect(container.querySelector('.spinner')).toBeTruthy();
  });

  it('uses the Turkish catalog', () => {
    store.set({ language: 'tr' });
    render(<ClosingOverlay active />);
    expect(screen.getByText('Pegg kapatılıyor…')).toBeTruthy();
  });
});
