import * as vscode from 'vscode';
import { SessionListItem } from '../acp/types';
import { ChatController } from '../chat/controller';
import { Logger } from '../logger';

export class SessionsTreeProvider implements vscode.TreeDataProvider<SessionTreeItem> {
  private readonly emitter = new vscode.EventEmitter<void>();
  readonly onDidChangeTreeData = this.emitter.event;

  constructor(
    private readonly controller: ChatController,
    private readonly log: Logger,
  ) {
    controller.on('sessions', () => this.refresh());
    controller.on('session', () => this.refresh());
    controller.on('runFinished', () => this.refresh());
  }

  refresh(): void {
    this.emitter.fire();
  }

  getTreeItem(element: SessionTreeItem): vscode.TreeItem {
    return element;
  }

  async getChildren(): Promise<SessionTreeItem[]> {
    try {
      const sessions = await this.controller.listSessions(true);
      return sessions.map((session) => new SessionTreeItem(session, session.id === this.controller.state.peggSessionId));
    } catch (err) {
      this.log.debug('Could not list sessions', err);
      return [];
    }
  }
}

export class SessionTreeItem extends vscode.TreeItem {
  override readonly contextValue = 'peggSession';

  constructor(
    readonly session: SessionListItem,
    readonly isCurrent: boolean,
  ) {
    super(session.title || 'Untitled session', vscode.TreeItemCollapsibleState.None);
    this.id = session.id;
    this.iconPath = new vscode.ThemeIcon(
      isCurrent ? 'comment-discussion' : 'history',
      isCurrent ? new vscode.ThemeColor('charts.green') : undefined,
    );
    this.description = [isCurrent ? 'current' : '', session.model, session.mode].filter(Boolean).join(' · ');
    this.tooltip = new vscode.MarkdownString(
      [
        `**${session.title}**`,
        '',
        `Project: \`${session.projectDir ?? 'unknown'}\``,
        `Model: \`${session.provider ?? ''}/${session.model ?? ''}\``,
        `Mode: \`${session.mode ?? 'auto'}\``,
        `Updated: ${new Date(session.updatedAt).toLocaleString()}`,
        isCurrent ? '\n**Current session**' : '',
      ].join('\n'),
    );
    this.command = {
      command: 'pegg.session.resume',
      title: 'Resume Session',
      arguments: [session.id],
    };
  }
}
