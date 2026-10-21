import * as vscode from 'vscode';
import { ChatController } from '../chat/controller';

export class PeggStatusBar {
  private readonly item: vscode.StatusBarItem;

  constructor(private readonly controller: ChatController) {
    this.item = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 90);
    this.item.command = 'pegg.chat.open';
    this.item.name = 'Pegg';
    this.item.show();
    controller.on('state', () => this.update());
    this.update();
  }

  update(): void {
    const state = this.controller.state;
    if (!state.connected && !state.acpSessionId) {
      this.item.text = '$(sparkle) Pegg';
      this.item.tooltip = 'Open Pegg chat';
      return;
    }
    if (state.running) {
      this.item.text = `$(sync~spin) Pegg · ${state.mode} · ${state.model ?? 'auto'}`;
      this.item.tooltip = 'Pegg is working. Click to open chat.';
      return;
    }
    if (state.error) {
      this.item.text = '$(error) Pegg';
      this.item.tooltip = state.error;
      return;
    }
    const usage = state.usage.used > 0 ? ` · ${compact(state.usage.used)} tokens` : '';
    this.item.text = `$(sparkle) Pegg · ${state.mode} · ${state.model ?? 'auto'}${usage}`;
    this.item.tooltip = [
      `Session: ${state.title}`,
      `Provider: ${state.provider ?? 'auto'}`,
      `Model: ${state.model ?? 'auto'}`,
      `Mode: ${state.mode}`,
      state.usage.cost !== undefined ? `Cost: $${state.usage.cost.toFixed(4)}` : '',
    ]
      .filter(Boolean)
      .join('\n');
  }

  dispose(): void {
    this.item.dispose();
  }
}

function compact(value: number): string {
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(1)}M`;
  if (value >= 1_000) return `${(value / 1_000).toFixed(1)}K`;
  return String(value);
}
