import * as vscode from 'vscode';

export type PeggMode = 'auto' | 'plan' | 'build';

export interface PeggConfig {
  executablePath: string;
  autoDownload: boolean;
  allowWorkspaceBinary: boolean;
  binaryVersion: string;
  downloadBaseUrl: string;
  provider: string;
  model: string;
  defaultMode: PeggMode;
  includeActiveFile: boolean;
  includeSelection: boolean;
  maxFileBytes: number;
  sendOnEnter: 'enter' | 'ctrl+enter';
  notifyOnComplete: boolean;
  env: Record<string, string>;
  logLevel: 'error' | 'warn' | 'info' | 'debug';
}

export function getConfig(): PeggConfig {
  const c = vscode.workspace.getConfiguration('pegg');
  return {
    executablePath: c.get<string>('executablePath', '').trim(),
    autoDownload: c.get<boolean>('binary.autoDownload', true),
    allowWorkspaceBinary: c.get<boolean>('binary.allowWorkspaceBinary', true),
    binaryVersion: c.get<string>('binary.version', 'match-extension').trim() || 'match-extension',
    downloadBaseUrl:
      c.get<string>('downloadBaseUrl', 'https://github.com/peggco/pegg/releases/download').trim() ||
      'https://github.com/peggco/pegg/releases/download',
    provider: c.get<string>('provider', '').trim(),
    model: c.get<string>('model', '').trim(),
    defaultMode: c.get<PeggMode>('defaultMode', 'auto'),
    includeActiveFile: c.get<boolean>('context.includeActiveFile', true),
    includeSelection: c.get<boolean>('context.includeSelection', true),
    maxFileBytes: Math.max(1024, c.get<number>('context.maxFileBytes', 262144)),
    sendOnEnter: c.get<'enter' | 'ctrl+enter'>('chat.sendOnEnter', 'enter'),
    notifyOnComplete: c.get<boolean>('notifications.onComplete', true),
    env: c.get<Record<string, string>>('env', {}) ?? {},
    logLevel: c.get<'error' | 'warn' | 'info' | 'debug'>('logLevel', 'info'),
  };
}

export function workspaceRoot(): string | undefined {
  const active = vscode.window.activeTextEditor?.document.uri;
  if (active) {
    const folder = vscode.workspace.getWorkspaceFolder(active);
    if (folder) return folder.uri.fsPath;
  }
  return vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
}
