import { createRoot } from 'react-dom/client';
import { App } from './App';
import { setHost, type ChatHost } from './host';
import './styles.css';

export function mount(chatHost: ChatHost): void {
  setHost(chatHost);
  const root = document.getElementById('root');
  if (root) createRoot(root).render(<App />);
}
