import { createRef } from 'react';
import { cleanup, fireEvent, render } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const { removeRecentFn, setWorkspaceFn, chooseWorkspaceFn } = vi.hoisted(() => ({
  removeRecentFn: vi.fn(async () => undefined),
  setWorkspaceFn: vi.fn(async () => undefined),
  chooseWorkspaceFn: vi.fn(async () => undefined),
}));

vi.mock('../state/actions', () => ({
  removeRecent: removeRecentFn,
  setWorkspace: setWorkspaceFn,
  chooseWorkspace: chooseWorkspaceFn,
}));

import { ProjectMenu } from './ProjectMenu';
import { store } from '../state/store';

beforeEach(() => {
  vi.clearAllMocks();
  store.set({
    language: 'en',
    workspace: '/a',
    recents: [
      { path: '/a', name: 'alpha', openedAt: '' },
      { path: '/b', name: 'beta', openedAt: '' },
    ],
  });
});

afterEach(() => {
  cleanup();
});

function openRowMenu(container: HTMLElement) {
  fireEvent.click(container.querySelectorAll('.project-more')[1]);
}

describe('ProjectMenu', () => {
  it('marks the open project and ignores clicks on its anchor', () => {
    const anchorRef = createRef<HTMLDivElement>();
    const onClose = vi.fn();
    const { container, getAllByRole } = render(
      <>
        <div ref={anchorRef} data-testid="anchor" />
        <ProjectMenu anchorRef={anchorRef} onClose={onClose} />
      </>,
    );

    expect(getAllByRole('menuitemradio')[0].getAttribute('aria-checked')).toBe('true');
    expect(getAllByRole('menuitemradio')[1].getAttribute('aria-checked')).toBe('false');

    fireEvent.mouseDown(anchorRef.current as HTMLElement);
    expect(onClose).not.toHaveBeenCalled();
    expect(container.querySelector('.project-menu')).not.toBeNull();
  });

  it('asks for confirmation before removing a project', () => {
    const onClose = vi.fn();
    const { container, getByText } = render(<ProjectMenu onClose={onClose} />);

    openRowMenu(container);
    fireEvent.click(getByText('Remove Local Project'));

    expect(removeRecentFn).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
    expect(getByText('Remove this project?')).toBeTruthy();
    expect(getByText('Nothing on disk is deleted.')).toBeTruthy();

    fireEvent.click(getByText('Remove'));

    expect(removeRecentFn).toHaveBeenCalledWith('/b');
    expect(onClose).toHaveBeenCalled();
  });

  it('keeps the submenu open when removal is cancelled', () => {
    const { container, getByText } = render(<ProjectMenu onClose={() => undefined} />);

    openRowMenu(container);
    fireEvent.click(getByText('Remove Local Project'));
    fireEvent.click(getByText('Cancel'));

    expect(removeRecentFn).not.toHaveBeenCalled();
    expect(container.querySelector('.project-row-menu')).not.toBeNull();
    expect(getByText('Remove Local Project')).toBeTruthy();
  });

  it('closes confirmation, then the submenu, then the menu on Escape', () => {
    const onClose = vi.fn();
    const { container, getByText } = render(<ProjectMenu onClose={onClose} />);

    openRowMenu(container);
    fireEvent.click(getByText('Remove Local Project'));
    fireEvent.keyDown(window, { key: 'Escape' });
    expect(getByText('Remove Local Project')).toBeTruthy();
    expect(onClose).not.toHaveBeenCalled();

    fireEvent.keyDown(window, { key: 'Escape' });
    expect(container.querySelector('.project-row-menu')).toBeNull();
    expect(onClose).not.toHaveBeenCalled();

    fireEvent.keyDown(window, { key: 'Escape' });
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('renders the menu in Turkish', () => {
    store.set({ language: 'tr' });
    const { container, getByText } = render(<ProjectMenu onClose={() => undefined} />);

    openRowMenu(container);
    fireEvent.click(getByText('Yerel Projeyi Kaldır'));

    expect(getByText('Bu proje kaldırılsın mı?')).toBeTruthy();
    expect(getByText('Kaldır')).toBeTruthy();
  });
});
