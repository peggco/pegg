import { mount } from '../../../chat-ui/src/webview/main';

interface VsCodeApi {
  postMessage(message: unknown): void;
  getState(): unknown;
  setState(state: unknown): void;
}

declare function acquireVsCodeApi(): VsCodeApi;

const api = acquireVsCodeApi();
mount({ postMessage: (message) => api.postMessage(message) });
