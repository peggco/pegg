import * as vscode from 'vscode';
import type { PluginPageViewHost } from '../views/pluginPageView';

export function registerPluginPageCommands(host: PluginPageViewHost): vscode.Disposable[] {
  return [
    vscode.commands.registerCommand('pegg.pluginPages.open', (owner?: string, pageId?: string) => {
      void host.open(owner, pageId).catch((err) => {
        void vscode.window.showErrorMessage(`Pegg: ${err instanceof Error ? err.message : String(err)}`);
      });
    }),
  ];
}
