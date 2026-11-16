export interface PluginPageHost {
  postMessage(message: unknown): void;
}

let current: PluginPageHost = { postMessage: () => undefined };

export function setPluginPageHost(next: PluginPageHost): void {
  current = next;
}

export const pluginPageHost: PluginPageHost = {
  postMessage: (message: unknown) => current.postMessage(message),
};
