import { mountPluginPage } from '../../chat-ui/src/pluginPage/main';
import './intellij.css';

mountPluginPage({
  postMessage: (message) => {
    if (window.__peggSend) {
      window.__peggSend(message);
    } else {
      (window.__peggOutbox ??= []).push(JSON.stringify(message));
    }
  },
});
