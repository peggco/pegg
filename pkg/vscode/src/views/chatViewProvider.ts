import * as vscode from 'vscode';
import { existsSync } from 'node:fs';
import { basename } from 'node:path';
import { ChatController } from '../chat/controller';
import { makeAttachment } from '../chat/context';
import { getConfig } from '../config';
import { Logger } from '../logger';

export class ChatViewProvider implements vscode.WebviewViewProvider {
  static readonly viewType = 'pegg.chat';
  private view: vscode.WebviewView | undefined;
  private sendTimer: NodeJS.Timeout | undefined;

  constructor(
    private readonly extensionUri: vscode.Uri,
    private readonly controller: ChatController,
    private readonly log: Logger,
  ) {
    controller.setUiVisible(() => this.view?.visible === true);
    controller.on('state', () => this.scheduleState());
    controller.on('composerText', (text: string) => this.postComposerText(text));
  }

  resolveWebviewView(view: vscode.WebviewView): void {
    this.view = view;
    view.webview.options = {
      enableScripts: true,
      localResourceRoots: [vscode.Uri.joinPath(this.extensionUri, 'dist')],
    };
    view.webview.html = this.html(view.webview);
    view.webview.onDidReceiveMessage((message) => void this.onMessage(message));
    view.onDidChangeVisibility(() => {
      if (view.visible) this.postState();
    });
    void this.controller.activate().catch((err) => this.log.error('Could not start Pegg', err));
  }

  async reveal(): Promise<void> {
    await vscode.commands.executeCommand('pegg.chat.focus');
    this.postState();
  }

  private html(webview: vscode.Webview): string {
    const nonce = nonceString();
    const script = webview.asWebviewUri(vscode.Uri.joinPath(this.extensionUri, 'dist', 'webview.js'));
    const style = webview.asWebviewUri(vscode.Uri.joinPath(this.extensionUri, 'dist', 'webview.css'));
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

  private async forkMessage(id: string): Promise<void> {
    try {
      await this.controller.forkMessage(id);
      await vscode.commands.executeCommand('pegg.session.refresh');
    } catch (err) {
      this.showActionError(err);
    }
  }

  private async revertMessage(id: string): Promise<void> {
    try {
      await this.controller.revertMessage(id);
    } catch (err) {
      this.showActionError(err);
    }
  }

  private showActionError(err: unknown): void {
    const message = err instanceof Error ? err.message : String(err);
    this.log.warn('Message action failed', err);
    void vscode.window.showErrorMessage(`Pegg: ${message}`);
  }

  private postComposerText(text: string): void {
    if (!this.view) return;
    void this.view.webview.postMessage({ type: 'composerText', text });
  }

  private scheduleState(): void {
    if (this.sendTimer) return;
    this.sendTimer = setTimeout(() => {
      this.sendTimer = undefined;
      this.postState();
    }, 24);
  }

  private postState(): void {
    if (!this.view) return;
    const cfg = getConfig();
    void this.view.webview.postMessage({
      type: 'state',
      state: this.controller.state,
      settings: {
        sendOnEnter: cfg.sendOnEnter,
        defaultMode: cfg.defaultMode,
      },
    });
  }

  private async onMessage(message: any): Promise<void> {
    try {
      switch (message?.type) {
        case 'ready':
          this.postState();
          void this.postFiles();
          return;
        case 'listFiles':
          void this.postFiles();
          return;
        case 'send':
          try {
            await this.controller.sendPrompt(String(message.text ?? ''), {
              attachments: this.controller.state.composerAttachments,
            });
          } catch (err) {
            this.log.warn('Prompt failed', err);
          }
          return;
        case 'stop':
          await this.controller.cancel();
          return;
        case 'forkMessage':
          await this.forkMessage(String(message.id ?? ''));
          return;
        case 'revertMessage':
          await this.revertMessage(String(message.id ?? ''));
          return;
        case 'newSession':
          await this.controller.newSession();
          return;
        case 'setMode':
          await this.controller.setMode(message.mode);
          return;
        case 'pickMode':
          await vscode.commands.executeCommand('pegg.chat.selectMode');
          return;
        case 'pickModel':
          await vscode.commands.executeCommand('pegg.chat.selectModel');
          return;
        case 'pickSession':
          await vscode.commands.executeCommand('pegg.session.resume');
          return;
        case 'pickFiles':
          await this.pickFiles();
          return;
        case 'removeAttachment':
          this.controller.removeComposerAttachment(String(message.id));
          return;
        case 'attachPaths': {
          for (const path of Array.isArray(message.paths) ? message.paths : []) {
            const attachment = await makeAttachment(String(path));
            if (attachment) this.controller.addComposerAttachment(attachment);
          }
          return;
        }
        case 'addAttachmentData': {
          const name = String(message.name ?? 'attachment');
          const mediaType = String(message.mediaType ?? 'application/octet-stream');
          this.controller.addComposerAttachment({
            id: `att-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`,
            name,
            mediaType,
            data: String(message.data ?? ''),
            size: Number(message.size ?? 0),
          });
          return;
        }
        case 'permissionResponse':
          this.controller.respondPermission(String(message.id), String(message.optionId));
          return;
        case 'questionResponse':
          this.controller.respondQuestion(String(message.id), message.answers ?? {});
          return;
        case 'startVoice': {
          try {
            await this.controller.startVoice();
            await this.view?.webview.postMessage({ type: 'recording', recording: true });
          } catch (err) {
            await this.view?.webview.postMessage({ type: 'recording', recording: false });
            await this.view?.webview.postMessage({
              type: 'voiceError',
              error: err instanceof Error ? err.message : String(err),
            });
            throw err;
          }
          return;
        }
        case 'stopVoice': {
          await this.view?.webview.postMessage({ type: 'recording', recording: false });
          try {
            const text = await this.controller.stopVoice();
            await this.view?.webview.postMessage({ type: 'transcript', text });
          } catch (err) {
            await this.view?.webview.postMessage({
              type: 'voiceError',
              error: err instanceof Error ? err.message : String(err),
            });
            throw err;
          }
          return;
        }
        case 'transcribe': {
          const text = await this.controller.transcribe(
            String(message.audio ?? ''),
            String(message.mediaType ?? 'audio/webm'),
            message.fileName,
          );
          await this.view?.webview.postMessage({ type: 'transcript', text });
          return;
        }
        case 'openFile':
          await openFile(String(message.path), Number(message.line ?? 0));
          return;
        case 'openDiff':
          await openDiff(String(message.path), String(message.oldText ?? ''), String(message.newText ?? ''));
          return;
        case 'openExternal': {
          const url = String(message.url ?? '');
          if (/^https?:\/\//i.test(url)) await vscode.env.openExternal(vscode.Uri.parse(url));
          return;
        }
        case 'openSettings':
          await vscode.commands.executeCommand('workbench.action.openSettings', 'pegg');
          return;
        case 'showLogs':
          this.log.show();
          return;
        case 'restart':
          await this.controller.restart();
          return;
        case 'copyText':
          await vscode.env.clipboard.writeText(String(message.text ?? ''));
          return;
        default:
          this.log.debug('Unknown webview message', message?.type);
      }
    } catch (err) {
      this.log.error('Webview action failed', err);
      void vscode.window.showErrorMessage(`Pegg: ${err instanceof Error ? err.message : String(err)}`);
    }
  }

  private async pickFiles(): Promise<void> {
    const uris = await vscode.window.showOpenDialog({
      canSelectMany: true,
      canSelectFiles: true,
      canSelectFolders: false,
      openLabel: 'Attach to Pegg',
    });
    for (const uri of uris ?? []) {
      if (uri.scheme !== 'file') continue;
      const attachment = await makeAttachment(uri.fsPath);
      if (attachment) this.controller.addComposerAttachment(attachment);
    }
  }

  private async postFiles(): Promise<void> {
    if (!this.view) return;
    const uris = await vscode.workspace.findFiles(
      '**/*',
      '**/{node_modules,.git,dist,build,out,site,.cache,coverage,.vscode-test}/**',
      5000,
    );
    const files = uris
      .map((uri) => vscode.workspace.asRelativePath(uri, false))
      .filter(Boolean)
      .sort((a, b) => a.localeCompare(b));
    await this.view.webview.postMessage({ type: 'files', files });
  }
}

async function openFile(path: string, line: number): Promise<void> {
  if (!path) return;
  const uri = vscode.Uri.file(path);
  if (!existsSync(path) && uri.scheme === 'file') {
    void vscode.window.showWarningMessage(`File not found: ${path}`);
    return;
  }
  const document = await vscode.workspace.openTextDocument(uri);
  const position = new vscode.Position(Math.max(0, line - 1), 0);
  await vscode.window.showTextDocument(document, {
    preview: true,
    selection: new vscode.Range(position, position),
  });
}

async function openDiff(path: string, oldText: string, newText: string): Promise<void> {
  const language = languageFor(path);
  const left = await vscode.workspace.openTextDocument({ content: oldText, language });
  const right = await vscode.workspace.openTextDocument({ content: newText, language });
  await vscode.commands.executeCommand(
    'vscode.diff',
    left.uri,
    right.uri,
    `Pegg: ${basename(path)}`,
    { preview: true },
  );
}

function languageFor(path: string): string {
  const ext = path.split('.').pop()?.toLowerCase() ?? '';
  const map: Record<string, string> = {
    ts: 'typescript',
    tsx: 'typescriptreact',
    js: 'javascript',
    jsx: 'javascriptreact',
    go: 'go',
    py: 'python',
    rs: 'rust',
    java: 'java',
    json: 'json',
    md: 'markdown',
    css: 'css',
    html: 'html',
    sh: 'shellscript',
    yaml: 'yaml',
    yml: 'yaml',
    toml: 'toml',
  };
  return map[ext] ?? 'plaintext';
}

function nonceString(): string {
  const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
  let out = '';
  for (let i = 0; i < 32; i++) out += chars[Math.floor(Math.random() * chars.length)];
  return out;
}
