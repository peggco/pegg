import * as vscode from 'vscode';
import { BinaryManager } from './binary/manager';
import { ChatController } from './chat/controller';
import { registerChatParticipant } from './chatParticipant';
import { registerChatCommands } from './commands/chat';
import { registerEditorCommands } from './commands/editor';
import { registerCodeActions } from './integrations/codeActions';
import { PeggStatusBar } from './integrations/statusBar';
import { Logger } from './logger';
import { ChatViewProvider } from './views/chatViewProvider';
import { SessionsTreeProvider } from './views/sessionsTree';

let controller: ChatController | undefined;
let log: Logger | undefined;

export async function activate(context: vscode.ExtensionContext): Promise<void> {
  log = new Logger();
  const binary = new BinaryManager(context, log);
  controller = new ChatController(context, binary, log);

  const chatProvider = new ChatViewProvider(context.extensionUri, controller, log);
  const sessionsProvider = new SessionsTreeProvider(controller, log);
  const statusBar = new PeggStatusBar(controller);

  context.subscriptions.push(
    log,
    { dispose: () => void controller?.dispose() },
    statusBar,
    vscode.window.registerWebviewViewProvider(ChatViewProvider.viewType, chatProvider, {
      webviewOptions: { retainContextWhenHidden: true },
    }),
    vscode.window.createTreeView('pegg.sessions', { treeDataProvider: sessionsProvider, showCollapseAll: false }),
    registerCodeActions(),
    registerChatParticipant(controller),
    ...registerChatCommands(controller, binary, log, sessionsProvider),
    ...registerEditorCommands(controller),
  );

  context.subscriptions.push(
    vscode.workspace.onDidChangeConfiguration((event) => {
      if (
        event.affectsConfiguration('pegg.executablePath') ||
        event.affectsConfiguration('pegg.binary') ||
        event.affectsConfiguration('pegg.env')
      ) {
        void vscode.window
          .showInformationMessage('Pegg binary settings changed. Restart the server now?', 'Restart')
          .then((choice) => {
            if (choice === 'Restart') void vscode.commands.executeCommand('pegg.restartServer');
          });
      }
      if (event.affectsConfiguration('pegg.logLevel')) log?.debug('Log level changed');
    }),
  );

  if (!vscode.workspace.isTrusted) {
    void vscode.window.showWarningMessage(
      'Pegg is disabled in untrusted workspaces because it can run commands and edit files.',
    );
  }

  log.info('Pegg extension activated');
}

export async function deactivate(): Promise<void> {
  await controller?.dispose();
  controller = undefined;
  log = undefined;
}
