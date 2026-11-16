import type {
  PluginCatalogItem,
  PluginCatalogResult,
  PluginInfo,
  PluginListResult,
  PluginMutationResult,
} from './types';

export interface PluginPageState {
  installed: PluginInfo[];
  catalog: PluginCatalogItem[];
  pluginsEnabled: boolean;
  loading: boolean;
  catalogLoading: boolean;
  error?: string;
  notice?: string;
  pending: Record<string, boolean>;
}

export type PluginStatus = 'loaded' | 'disabled' | 'error' | 'not-loaded';

export function initialPluginPageState(): PluginPageState {
  return {
    installed: [],
    catalog: [],
    pluginsEnabled: true,
    loading: false,
    catalogLoading: false,
    pending: {},
  };
}

export function pluginStatus(plugin: PluginInfo): PluginStatus {
  if (plugin.lastError) return 'error';
  if (!plugin.enabled) return 'disabled';
  if (plugin.loaded) return 'loaded';
  return 'not-loaded';
}

export function pluginStatusLabel(plugin: PluginInfo): string {
  switch (pluginStatus(plugin)) {
    case 'loaded':
      return 'Loaded';
    case 'disabled':
      return 'Disabled';
    case 'error':
      return 'Failed';
    default:
      return 'Not loaded';
  }
}

export function applyPluginList(state: PluginPageState, result: PluginListResult): PluginPageState {
  return {
    ...state,
    installed: [...(result.plugins ?? [])].sort((a, b) => a.name.localeCompare(b.name)),
    pluginsEnabled: result.pluginsEnabled ?? state.pluginsEnabled,
    loading: false,
    error: undefined,
  };
}

export function applyPluginCatalog(state: PluginPageState, result: PluginCatalogResult): PluginPageState {
  return {
    ...state,
    catalog: [...(result.items ?? [])].sort((a, b) => a.name.localeCompare(b.name)),
    catalogLoading: false,
    error: undefined,
  };
}

export function setPluginPending(
  state: PluginPageState,
  requestId: string,
  pending: boolean,
): PluginPageState {
  const next = { ...state.pending };
  if (pending) next[requestId] = true;
  else delete next[requestId];
  return { ...state, pending: next };
}

export function applyPluginAction(
  state: PluginPageState,
  result: PluginMutationResult,
): PluginPageState {
  let notice = 'Plugin changes applied.';
  if (result.restartRequired) {
    notice = 'Restart the Pegg server to apply plugin changes.';
  }
  if (result.warnings?.length) {
    notice = `${notice} ${result.warnings.join(' ')}`;
  }
  return { ...state, pending: {}, notice, error: undefined };
}

export function applyPluginError(
  state: PluginPageState,
  message: string,
  catalog = false,
): PluginPageState {
  return {
    ...state,
    pending: {},
    loading: catalog ? state.loading : false,
    catalogLoading: catalog ? false : state.catalogLoading,
    error: message,
  };
}

export function pluginEnvKeys(plugin: PluginInfo, item?: PluginCatalogItem): string[] {
  const keys = new Set<string>(plugin.envKeys ?? []);
  for (const key of item?.requiredEnv ?? []) keys.add(key);
  return Array.from(keys).sort((a, b) => a.localeCompare(b));
}

export function pluginIsBusy(state: PluginPageState, key: string): boolean {
  return Boolean(state.pending[key]);
}
