import * as vscode from 'vscode';
import { homedir } from 'node:os';
import { join } from 'node:path';
import { ChatController } from '../chat/controller';
import { BinaryManager } from '../binary/manager';
import { Logger } from '../logger';
import { SessionMode } from '../acp/types';
import { SessionsTreeProvider } from '../views/sessionsTree';

export function registerChatCommands(
  controller: ChatController,
  binary: BinaryManager,
  log: Logger,
  sessions: SessionsTreeProvider,
): vscode.Disposable[] {
  return [
    vscode.commands.registerCommand('pegg.chat.open', async () => {
      await vscode.commands.executeCommand('pegg.chat.focus');
      await controller.activate().catch((err) => showError(err));
    }),
    vscode.commands.registerCommand('pegg.chat.newSession', async () => {
      await vscode.commands.executeCommand('pegg.chat.focus');
      await controller.newSession().catch((err) => showError(err));
      sessions.refresh();
    }),
    vscode.commands.registerCommand('pegg.chat.stop', () => controller.cancel()),
    vscode.commands.registerCommand('pegg.chat.clear', async () => {
      const confirm = await vscode.window.showWarningMessage('Clear the current chat view?', { modal: true }, 'Clear');
      if (confirm !== 'Clear') return;
      await controller.newSession().catch((err) => showError(err));
      sessions.refresh();
    }),
    vscode.commands.registerCommand('pegg.chat.selectMode', async () => {
      const picked = await vscode.window.showQuickPick(
        [
          { label: 'Auto', description: 'Let Pegg plan when useful', value: 'auto' },
          { label: 'Plan', description: 'Read-only planning before changes', value: 'plan' },
          { label: 'Build', description: 'Execute changes immediately', value: 'build' },
        ],
        { title: 'Pegg mode', placeHolder: 'Choose a mode' },
      );
      if (picked) await controller.setMode(picked.value as SessionMode).catch((err) => showError(err));
    }),
    vscode.commands.registerCommand('pegg.chat.selectModel', async () => {
      try {
        const models = await controller.models();
        if (models.length === 0) {
          void vscode.window.showInformationMessage('No Pegg models are configured. Run "Pegg: Provider Login".');
          return;
        }
        const picked = await vscode.window.showQuickPick(
          models.map((entry) => ({
            label: entry.model.name || entry.model.id,
            description: entry.provider,
            detail: entry.model.id,
            provider: entry.provider,
            model: entry.model.id,
          })),
          { title: 'Pegg model', placeHolder: 'Choose a provider and model', matchOnDescription: true },
        );
        if (picked) await controller.setModel(picked.provider, picked.model).catch((err) => showError(err));
      } catch (err) {
        showError(err);
      }
    }),
    vscode.commands.registerCommand('pegg.session.resume', async (sessionId?: string) => {
      let id = sessionId;
      if (!id) {
        const sessionsList = await controller.listSessions(true).catch(() => []);
        const picked = await vscode.window.showQuickPick(
          sessionsList.map((session) => ({
            label: session.title || 'Untitled session',
            description: [
              session.id === controller.state.peggSessionId ? 'Current' : '',
              session.model,
              session.mode,
            ]
              .filter(Boolean)
              .join(' · '),
            detail: session.projectDir,
            id: session.id,
          })),
          { title: 'Pegg sessions', matchOnDescription: true, matchOnDetail: true },
        );
        id = picked?.id;
      }
      if (!id) return;
      await vscode.commands.executeCommand('pegg.chat.focus');
      await controller.resumeSession(id).catch((err) => showError(err));
    }),
    vscode.commands.registerCommand('pegg.session.delete', async (arg?: string | { session?: { id?: string } }) => {
      let id = typeof arg === 'string' ? arg : arg?.session?.id;
      if (!id) {
        const sessionsList = await controller.listSessions(true).catch(() => []);
        const picked = await vscode.window.showQuickPick(
          sessionsList.map((session) => ({ label: session.title, description: session.id, id: session.id })),
          { title: 'Delete a Pegg session' },
        );
        id = picked?.id;
      }
      if (!id) return;
      const confirm = await vscode.window.showWarningMessage(
        'Delete this Pegg session permanently?',
        { modal: true },
        'Delete',
      );
      if (confirm !== 'Delete') return;
      await controller.deleteSession(id).catch((err) => showError(err));
      sessions.refresh();
    }),
    vscode.commands.registerCommand('pegg.session.rename', async (arg?: string | { session?: { id?: string } }) => {
      let id = typeof arg === 'string' ? arg : arg?.session?.id;
      if (!id) {
        const sessionsList = await controller.listSessions(true).catch(() => []);
        const picked = await vscode.window.showQuickPick(
          sessionsList.map((session) => ({ label: session.title, description: session.id, id: session.id })),
          { title: 'Rename a Pegg session' },
        );
        id = picked?.id;
      }
      if (!id) return;
      const current =
        (await controller.listSessions(true).catch(() => [])).find((session) => session.id === id)?.title ?? '';
      const title = await vscode.window.showInputBox({
        title: 'Rename Pegg session',
        value: current,
        validateInput: (value) => (value.trim() ? undefined : 'Title is required'),
      });
      if (!title) return;
      await controller.renameSession(id, title.trim()).catch((err) => showError(err));
      sessions.refresh();
    }),
    vscode.commands.registerCommand('pegg.session.refresh', () => sessions.refresh()),
    vscode.commands.registerCommand('pegg.login', async () => runBinary(binary, ['login'], 'Pegg Login')),
    vscode.commands.registerCommand('pegg.doctor', async () => runBinary(binary, ['doctor'], 'Pegg Doctor')),
    vscode.commands.registerCommand('pegg.update', async () => runBinary(binary, ['update'], 'Pegg Update')),
    vscode.commands.registerCommand('pegg.team', async () => runBinary(binary, ['team'], 'Pegg Team')),
    vscode.commands.registerCommand('pegg.openGlobalConfig', async () => {
      await openConfig(join(homedir(), '.pegg', 'pegg.json'));
    }),
    vscode.commands.registerCommand('pegg.openProjectConfig', async () => {
      const root = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
      if (!root) {
        void vscode.window.showWarningMessage('Open a folder first.');
        return;
      }
      const dir = vscode.Uri.file(join(root, '.pegg'));
      await vscode.workspace.fs.createDirectory(dir);
      await vscode.commands.executeCommand('revealInExplorer', dir);
    }),
    vscode.commands.registerCommand('pegg.showLogs', () => log.show()),
    vscode.commands.registerCommand('pegg.restartServer', async () => {
      await controller.restart().catch((err) => showError(err));
      sessions.refresh();
    }),
    vscode.commands.registerCommand('pegg.installBinary', async () => {
      try {
        const path = await vscode.window.withProgress(
          { location: vscode.ProgressLocation.Notification, title: 'Installing Pegg', cancellable: false },
          (progress) =>
            binary.ensure({
              force: true,
              progress: (message, increment) => progress.report({ message, increment }),
            }),
        );
        void vscode.window.showInformationMessage(`Pegg binary ready: ${path}`);
      } catch (err) {
        showError(err);
      }
    }),
  ];
}

async function runBinary(binary: BinaryManager, args: string[], title: string): Promise<void> {
  try {
    const path = await binary.ensure();
    const terminal = vscode.window.createTerminal({ name: title, shellPath: path, shellArgs: args });
    terminal.show();
  } catch (err) {
    showError(err);
  }
}

async function openConfig(path: string): Promise<void> {
  const uri = vscode.Uri.file(path);
  try {
    await vscode.workspace.fs.stat(uri);
  } catch {
    await vscode.workspace.fs.createDirectory(vscode.Uri.file(join(path, '..')));
    await vscode.workspace.fs.writeFile(uri, Buffer.from('{}\n', 'utf8'));
  }
  await vscode.window.showTextDocument(uri, { preview: false });
}

export function showError(err: unknown): void {
  const message = err instanceof Error ? err.message : String(err);
  void vscode.window.showErrorMessage(`Pegg: ${message}`);
}
