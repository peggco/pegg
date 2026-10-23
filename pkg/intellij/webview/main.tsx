import { mount } from '../../chat-ui/src/webview/main';
import './intellij.css';

mount({
  postMessage: (message) => {
    if (window.__peggSend) {
      window.__peggSend(message);
    } else {
      (window.__peggOutbox ??= []).push(JSON.stringify(message));
    }
  },
});
