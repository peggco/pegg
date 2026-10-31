import '@fontsource-variable/inter';
import '@fontsource-variable/jetbrains-mono';

import { Component, type ErrorInfo, type ReactNode } from 'react';
import { createRoot } from 'react-dom/client';
import { App } from './App';

import './styles/tokens.css';
import './styles/base.css';
import './styles/app.css';
import './styles/welcome.css';
import './styles/composer.css';
import './styles/chat.css';
import './styles/panels.css';
import './styles/markdown.css';
import './styles/overlays.css';
import './styles/settings.css';
import './styles/tools.css';

class ErrorBoundary extends Component<{ children: ReactNode }, { error?: Error }> {
  state: { error?: Error } = {};

  static getDerivedStateFromError(error: Error) {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    const message = `${error.message}\n${info.componentStack ?? ''}`;
    void (window as unknown as { go?: { main?: { App?: { Log?: (value: string) => Promise<void> } } } })
      .go?.main?.App?.Log?.(`ui crash: ${message}`)
      .catch(() => undefined);
  }

  render() {
    if (this.state.error) {
      return (
        <pre
          style={{
            margin: 0,
            padding: 24,
            whiteSpace: 'pre-wrap',
            fontFamily: 'monospace',
            fontSize: 12,
            color: '#fca5a5',
            background: '#18181b',
          }}
        >
          {String(this.state.error.stack ?? this.state.error.message)}
        </pre>
      );
    }
    return this.props.children;
  }
}

createRoot(document.getElementById('root')!).render(
  <ErrorBoundary>
    <App />
  </ErrorBoundary>,
);
