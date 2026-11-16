import * as vscode from 'vscode';
import type { PluginPage, PluginPageOutgoing } from '../../../chat-ui/src/pluginPage/types';
import type { ChatController } from '../chat/controller';
import type { Logger } from '../logger';
import { nonceString } from './nonce';

interface PanelState {
  panel: vscode.WebviewPanel;
  owner: string;
  pageId: string;
  page?: PluginPage;
  pages: PluginPage[];
}

export class PluginPageViewHost {
  private readonly panels = new Map<string, PanelState>();

  constructor(
    private readonly extensionUri: vscode.Uri,
    private readonly controller: ChatController,
    private readonly log: Logger,
  ) {}

  dispose(): void {
    for (const state of this.panels.values()) state.panel.dispose();
    this.panels.clear();
  }

  async open(owner?: string, pageId?: string): Promise<void> {
    if (owner && pageId) {
      const page = await this.controller.getPluginPage(owner, pageId);
      this.show(page.owner, page.id, page);
      return;
    }

    const pages = await this.controller.listPluginPages();
    if (!pages.length) {
      await vscode.window.showInformationMessage(
        'No plugin pages are available. Install a plugin that provides a page, then try again.',
      );
      return;
    }
    if (pages.length === 1) {
      this.show(pages[0].owner, pages[0].id, pages[0]);
      return;
    }

    const picked = await vscode.window.showQuickPick(
      pages.map((page) => ({
        label: page.title,
        description: page.owner,
        detail: page.description,
        page,
      })),
      { title: 'Pegg plugin pages', placeHolder: 'Open a plugin page' },
    );
    if (picked) this.show(picked.page.owner, picked.page.id, picked.page);
  }

  show(owner: string, pageId: string, page?: PluginPage): void {
    const key = `${owner}:${pageId}`;
    const existing = this.panels.get(key);
    if (existing) {
      existing.page = page ?? existing.page;
      existing.panel.reveal();
      void this.pushInit(existing);
      return;
    }

    const panel = vscode.window.createWebviewPanel(
      'pegg.pluginPage',
      page?.title || 'Plugin page',
      vscode.ViewColumn.Active,
      {
        enableScripts: true,
        retainContextWhenHidden: true,
        localResourceRoots: [vscode.Uri.joinPath(this.extensionUri, 'dist')],
      },
    );
    const state: PanelState = {
      panel,
      owner,
      pageId,
      page,
      pages: page ? [page] : [],
    };
    this.panels.set(key, state);

    panel.webview.html = this.html(panel.webview);
    panel.onDidDispose(() => this.panels.delete(key));
    panel.webview.onDidReceiveMessage((message: PluginPageOutgoing) => {
      void this.onMessage(state, message).catch((err) => this.log.error('Plugin page message failed', err));
    });
  }

  private async onMessage(state: PanelState, message: PluginPageOutgoing): Promise<void> {
    switch (message?.type) {
      case 'ready':
        await this.pushInit(state);
        break;
      case 'list': {
        try {
          const pages = await this.controller.listPluginPages();
          state.pages = pages;
          await this.post(state, { type: 'pages', requestId: message.requestId, pages });
        } catch (err) {
          await this.post(state, { type: 'error', requestId: message.requestId, error: errorText(err) });
        }
        break;
      }
      case 'get':
      case 'refresh': {
        try {
          const page = await this.controller.getPluginPage(message.owner, message.pageId);
          state.page = page;
          state.pages = upsert(state.pages, page);
          state.panel.title = page.title || 'Plugin page';
          await this.post(state, { type: 'page', requestId: message.requestId, page });
        } catch (err) {
          await this.post(state, { type: 'error', requestId: message.requestId, error: errorText(err) });
        }
        break;
      }
      case 'action': {
        try {
          const result = await this.controller.runPluginPageAction(
            message.owner,
            message.pageId,
            message.actionId,
            message.values,
          );
          if (result.page) {
            state.page = result.page;
            state.pages = upsert(state.pages, result.page);
            state.panel.title = result.page.title || 'Plugin page';
          }
          await this.post(state, {
            type: 'actionResult',
            requestId: message.requestId,
            output: result.output,
            page: result.page,
          });
        } catch (err) {
          await this.post(state, {
            type: 'actionResult',
            requestId: message.requestId,
            error: errorText(err),
          });
        }
        break;
      }
    }
  }

  private async pushInit(state: PanelState): Promise<void> {
    try {
      const page = await this.controller.getPluginPage(state.owner, state.pageId);
      const pages = await this.controller.listPluginPages().catch(() => state.pages);
      state.page = page;
      state.pages = upsert(pages, page);
      state.panel.title = page.title || 'Plugin page';
      await this.post(state, { type: 'init', pages: state.pages, page });
    } catch (err) {
      this.log.error('Could not open plugin page', err);
      await this.post(state, { type: 'error', error: errorText(err) });
    }
  }

  private async post(state: PanelState, message: unknown): Promise<void> {
    try {
      await state.panel.webview.postMessage(message);
    } catch (err) {
      this.log.debug('Plugin page post failed', err);
    }
  }

  private html(webview: vscode.Webview): string {
    const nonce = nonceString();
    const version = String(Date.now());
    const script = webview.asWebviewUri(
      vscode.Uri.joinPath(this.extensionUri, 'dist', 'pluginPage.js').with({ query: `v=${version}` }),
    );
    const style = webview.asWebviewUri(
      vscode.Uri.joinPath(this.extensionUri, 'dist', 'pluginPage.css').with({ query: `v=${version}` }),
    );
    const csp = [
      "default-src 'none'",
      `img-src ${webview.cspSource} data: https:`,
      `style-src ${webview.cspSource} 'unsafe-inline'`,
      `font-src ${webview.cspSource}`,
      `script-src 'nonce-${nonce}'`,
    ].join('; ');
    return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <meta http-equiv="Content-Security-Policy" content="${csp}" />
  <link rel="stylesheet" href="${style}" />
  <title>Pegg</title>
</head>
<body>
  <div id="root"></div>
  <script nonce="${nonce}" src="${script}"></script>
</body>
</html>`;
  }
}

function upsert(pages: PluginPage[], page: PluginPage): PluginPage[] {
  const index = pages.findIndex((candidate) => candidate.owner === page.owner && candidate.id === page.id);
  if (index < 0) return [...pages, page];
  const next = pages.slice();
  next[index] = page;
  return next;
}

function errorText(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
