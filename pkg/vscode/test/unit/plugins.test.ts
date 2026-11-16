import { describe, expect, it } from 'vitest';
import {
  applyPluginAction,
  applyPluginCatalog,
  applyPluginError,
  applyPluginList,
  initialPluginPageState,
  pluginEnvKeys,
  pluginStatus,
  pluginStatusLabel,
  setPluginPending,
} from '../../../chat-ui/src/plugins';

describe('plugin page state', () => {
  it('lists installed plugins sorted and clears the loading flag', () => {
    const state = applyPluginList(initialPluginPageState(), {
      pluginsEnabled: true,
      plugins: [
        { name: 'zulu', enabled: true, loaded: true },
        { name: 'alpha', enabled: false, loaded: false },
      ],
    });

    expect(state.installed.map((plugin) => plugin.name)).toEqual(['alpha', 'zulu']);
    expect(state.loading).toBe(false);
    expect(state.error).toBeUndefined();
  });

  it('derives a status for loaded, disabled, failed and not-loaded plugins', () => {
    expect(pluginStatus({ name: 'a', enabled: true, loaded: true })).toBe('loaded');
    expect(pluginStatus({ name: 'a', enabled: false, loaded: true })).toBe('disabled');
    expect(pluginStatus({ name: 'a', enabled: true, loaded: false, lastError: 'boom' })).toBe('error');
    expect(pluginStatus({ name: 'a', enabled: true, loaded: false })).toBe('not-loaded');
    expect(pluginStatusLabel({ name: 'a', enabled: true, loaded: false })).toBe('Not loaded');
  });

  it('merges catalog results and tracks pending actions', () => {
    let state = applyPluginCatalog(initialPluginPageState(), {
      items: [
        { id: 'peggco/zulu', name: 'zulu', source: 'peggco/zulu', updateAvailable: true },
        { id: 'peggco/alpha', name: 'alpha', source: 'peggco/alpha' },
      ],
    });

    expect(state.catalog.map((item) => item.name)).toEqual(['alpha', 'zulu']);
    expect(state.catalogLoading).toBe(false);

    state = setPluginPending(state, 'alpha', true);
    expect(state.pending.alpha).toBe(true);
    state = setPluginPending(state, 'alpha', false);
    expect(state.pending.alpha).toBeUndefined();
  });

  it('surfaces a restart notice after a mutation', () => {
    const state = applyPluginAction(initialPluginPageState(), {
      restartRequired: true,
      warnings: ['catalog may be stale'],
    });

    expect(state.notice).toContain('Restart the Pegg server');
    expect(state.notice).toContain('catalog may be stale');
    expect(state.error).toBeUndefined();
  });

  it('records errors and resets loading flags', () => {
    const loading = { ...initialPluginPageState(), loading: true, catalogLoading: true };
    const listError = applyPluginError(loading, 'list failed');
    expect(listError.error).toBe('list failed');
    expect(listError.loading).toBe(false);
    expect(listError.catalogLoading).toBe(true);

    const catalogError = applyPluginError(loading, 'catalog failed', true);
    expect(catalogError.loading).toBe(true);
    expect(catalogError.catalogLoading).toBe(false);
  });

  it('merges required env keys with configured keys', () => {
    const keys = pluginEnvKeys(
      { name: 'kanban', enabled: true, loaded: true, envKeys: ['KANBAN_BOARD'] },
      { id: 'peggco/kanban', name: 'kanban', source: 'peggco/kanban', requiredEnv: ['KANBAN_TOKEN'] },
    );
    expect(keys).toEqual(['KANBAN_BOARD', 'KANBAN_TOKEN']);
  });
});
