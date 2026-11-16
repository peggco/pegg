import { mountPluginPage } from '../../../chat-ui/src/pluginPage/main';

interface VsCodeApi {
  postMessage(message: unknown): void;
  getState(): unknown;
  setState(state: unknown): void;
}

declare function acquireVsCodeApi(): VsCodeApi;

const api = acquireVsCodeApi();
mountPluginPage({ postMessage: (message) => api.postMessage(message) });
