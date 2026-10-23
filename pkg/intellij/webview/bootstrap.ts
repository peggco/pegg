export {};

interface CefQuery {
  request: string;
  onSuccess?: (response: string) => void;
  onFailure?: (code: number, message: string) => void;
}

declare global {
  interface Window {
    __peggSend?: (message: unknown) => void;
    cefQuery?: (query: CefQuery) => void;
    __peggOutbox?: string[];
    __peggDrainOutbox?: () => void;
    __peggTheme?: (vars: Record<string, string>) => void;
    __peggHideReady?: boolean;
    __peggInlinePickers?: boolean;
  }
}

const outbox: string[] = [];
window.__peggOutbox = outbox;

function send(message: unknown): void {
  const request = JSON.stringify(message);
  if (typeof window.cefQuery === 'function') {
    window.cefQuery({ request });
  } else {
    outbox.push(request);
  }
}

window.__peggDrainOutbox = () => {
  if (typeof window.cefQuery !== 'function') return;
  const pending = outbox.splice(0, outbox.length);
  for (const request of pending) window.cefQuery({ request });
};

window.__peggTheme = (vars: Record<string, string>) => {
  const root = document.documentElement;
  for (const [name, value] of Object.entries(vars)) root.style.setProperty(name, value);
};

window.addEventListener('error', (event) => {
  console.error('PEGG_WINDOW_ERROR', event.message, event.filename, event.lineno, event.colno);
});

window.addEventListener('unhandledrejection', (event) => {
  console.error('PEGG_UNHANDLED_REJECTION', String(event.reason));
});

window.__peggHideReady = true;
window.__peggInlinePickers = true;
window.__peggSend = send;
