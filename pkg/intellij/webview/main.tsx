import { createRoot } from 'react-dom/client';
import { App } from '../../vscode/src/webview/App';
import '../../vscode/src/webview/styles.css';
import './intellij.css';

const root = document.getElementById('root');
if (root) createRoot(root).render(<App />);
