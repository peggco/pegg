import { beforeEach, describe, expect, it } from 'vitest';
import * as vscode from 'vscode';
import { createdPanels } from '../fixtures/vscode';
import type { ChatController } from '../../src/chat/controller';
import type { Logger } from '../../src/logger';
import { PluginPageViewHost } from '../../src/views/pluginPageView';
import type { PluginPage } from '../../../chat-ui/src/pluginPage/types';

const page: PluginPage = {
  id: 'kanban',
  title: 'Kanban',
  owner: 'kanban',
  sections: [
    {
      title: 'Board',
      kind: 'board',
      items: [{ id: 'card-1', title: 'Ship it', column: 'backlog', action: 'card.update', meta: { runAction: 'card.run' } }],
    },
  ],
  actions: [{ id: 'card.add', label: 'Add card' }],
};

function fakeController(): ChatController {
  return {
    listPluginPages: async () => [page],
    getPluginPage: async () => page,
    runPluginPageAction: async () => ({ output: 'moved', page: { ...page, title: 'Kanban updated' } }),
  } as unknown as ChatController;
}

const log = { error() {}, debug() {} } as unknown as Logger;

const flush = () => new Promise((resolve) => setTimeout(resolve, 0));

describe('PluginPageViewHost', () => {
  beforeEach(() => {
    createdPanels.length = 0;
  });

  it('renders a panel with CSP and the plugin page bundle', () => {
    const host = new PluginPageViewHost(vscode.Uri.file('/ext'), fakeController(), log);
    host.show('kanban', 'kanban', page);

    expect(createdPanels).toHaveLength(1);
    const panel = createdPanels[0];
    expect(panel.title).toBe('Kanban');
    expect(panel.webview.html).toContain("Content-Security-Policy");
    expect(panel.webview.html).toContain('pluginPage.js');
    expect(panel.webview.html).toMatch(/nonce-[A-Za-z0-9]+/);
    expect(panel.webview.options).toMatchObject({ enableScripts: true, retainContextWhenHidden: true });
    host.dispose();
  });

  it('sends the page list and page on ready', async () => {
    const host = new PluginPageViewHost(vscode.Uri.file('/ext'), fakeController(), log);
    host.show('kanban', 'kanban', page);
    const panel = createdPanels[0];

    panel.webview.emit({ type: 'ready' });
    await flush();
    await flush();

    const init = panel.webview.messages.find((message) => (message as { type?: string }).type === 'init');
    expect(init).toMatchObject({ type: 'init', page: { owner: 'kanban', id: 'kanban' } });
    host.dispose();
  });

  it('routes page actions and returns the updated page', async () => {
    const host = new PluginPageViewHost(vscode.Uri.file('/ext'), fakeController(), log);
    host.show('kanban', 'kanban', page);
    const panel = createdPanels[0];

    panel.webview.emit({
      type: 'action',
      requestId: 'req-1',
      owner: 'kanban',
      pageId: 'kanban',
      actionId: 'card.add',
      values: {},
    });
    await flush();
    await flush();

    const result = panel.webview.messages.find(
      (message) => (message as { type?: string; requestId?: string }).requestId === 'req-1',
    );
    expect(result).toMatchObject({
      type: 'actionResult',
      output: 'moved',
      page: { title: 'Kanban updated' },
    });
    expect(panel.title).toBe('Kanban updated');
    host.dispose();
  });

  it('reuses an existing panel for the same page', () => {
    const host = new PluginPageViewHost(vscode.Uri.file('/ext'), fakeController(), log);
    host.show('kanban', 'kanban', page);
    host.show('kanban', 'kanban', page);

    expect(createdPanels).toHaveLength(1);
    expect(createdPanels[0].revealCount).toBe(1);
    host.dispose();
    expect(createdPanels[0].disposed).toBe(true);
  });
});
