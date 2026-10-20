import { EventEmitter } from 'node:events';

type Values = Record<string, unknown>;

const values: Values = {
  'pegg.executablePath': process.env.PEGG_TEST_BINARY ?? '',
  'pegg.binary.autoDownload': false,
  'pegg.binary.allowWorkspaceBinary': true,
  'pegg.binary.version': 'match-extension',
  'pegg.downloadBaseUrl': 'https://github.com/peggco/pegg/releases/download',
  'pegg.provider': '',
  'pegg.model': '',
  'pegg.defaultMode': 'auto',
  'pegg.context.includeActiveFile': true,
  'pegg.context.includeSelection': true,
  'pegg.context.maxFileBytes': 262144,
  'pegg.chat.sendOnEnter': 'enter',
  'pegg.notifications.onComplete': false,
  'pegg.env': {},
  'pegg.logLevel': 'error',
};

export function __setConfig(key: string, value: unknown): void {
  values[key] = value;
}

const disposable = { dispose() {} };
const output = {
  debug() {},
  info() {},
  warn() {},
  error() {},
  show() {},
  dispose() {},
};

function workspaceRoot(): string {
  return process.env.PEGG_TEST_WORKSPACE ?? process.cwd();
}

export const Uri = {
  file: (path: string) => ({ fsPath: path, scheme: 'file', path, toString: () => `file://${path}` }),
  parse: (value: string) => ({ fsPath: value, scheme: 'file', path: value, toString: () => value }),
  joinPath: (base: { fsPath: string }, ...parts: string[]) => {
    const joined = [base.fsPath, ...parts].join('/').replace(/\/+/g, '/');
    return { fsPath: joined, scheme: 'file', path: joined, toString: () => `file://${joined}` };
  },
};

export class ThemeIcon {
  constructor(
    readonly id: string,
    readonly color?: unknown,
  ) {}
}

export class ThemeColor {
  constructor(readonly id: string) {}
}

export class TreeItem {
  label?: string;
  collapsibleState?: number;
  id?: string;
  iconPath?: unknown;
  description?: string;
  tooltip?: unknown;
  command?: unknown;
  contextValue?: string;
  constructor(label: string, state?: number) {
    this.label = label;
    this.collapsibleState = state;
  }
}

export const TreeItemCollapsibleState = { None: 0, Collapsed: 1, Expanded: 2 };
export const StatusBarAlignment = { Left: 1, Right: 2 };
export const ProgressLocation = { Notification: 15 };
export const CodeActionKind = { QuickFix: { value: 'quickfix' }, RefactorRewrite: { value: 'refactor.rewrite' } };
export class CodeAction {
  diagnostics?: unknown;
  command?: unknown;
  constructor(
    readonly title: string,
    readonly kind: unknown,
  ) {}
}

export const window = {
  state: { focused: true },
  activeTextEditor: undefined,
  createOutputChannel: () => output,
  registerWebviewViewProvider: () => disposable,
  createTreeView: () => disposable,
  createStatusBarItem: () => ({ show() {}, dispose() {} }),
  showInformationMessage: async () => undefined,
  showWarningMessage: async () => undefined,
  showErrorMessage: async () => undefined,
  showQuickPick: async () => undefined,
  showInputBox: async () => undefined,
  showOpenDialog: async () => undefined,
  showTextDocument: async () => ({}),
  createTerminal: () => ({ show() {} }),
};

export const workspace = {
  isTrusted: true,
  textDocuments: [],
  get workspaceFolders() {
    return [{ uri: Uri.file(workspaceRoot()), name: 'workspace', index: 0 }];
  },
  getConfiguration: (section: string) => ({
    get: (key: string, fallback: unknown) => values[`${section}.${key}`] ?? fallback,
  }),
  onDidChangeConfiguration: () => disposable,
  getWorkspaceFolder: () => undefined,
  findFiles: async () => [],
  asRelativePath: (value: string | { fsPath: string }) => (typeof value === 'string' ? value : value.fsPath),
  openTextDocument: async () => ({}),
  fs: {
    readFile: async () => new Uint8Array(),
    stat: async () => ({ size: 0 }),
    writeFile: async () => undefined,
    createDirectory: async () => undefined,
  },
};

export const languages = { registerCodeActionsProvider: () => disposable };
export const chat = { createChatParticipant: () => ({ ...disposable, iconPath: undefined }) };
export const commands = { registerCommand: () => disposable, executeCommand: async () => undefined };
export const env = { clipboard: { writeText: async () => undefined }, openExternal: async () => undefined };

export { EventEmitter };
