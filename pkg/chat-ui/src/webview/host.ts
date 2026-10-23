export interface ChatHost {
  postMessage(message: unknown): void;
}

let current: ChatHost = { postMessage: () => undefined };

export function setHost(next: ChatHost): void {
  current = next;
}

export const host: ChatHost = {
  postMessage: (message: unknown) => current.postMessage(message),
};
