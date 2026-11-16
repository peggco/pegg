import { createRoot } from 'react-dom/client';
import { App } from './App';
import { setPluginPageHost, type PluginPageHost } from './host';
import './styles.css';

export function mountPluginPage(host: PluginPageHost): void {
  setPluginPageHost(host);
  const root = document.getElementById('root');
  if (root) createRoot(root).render(<App />);
}
