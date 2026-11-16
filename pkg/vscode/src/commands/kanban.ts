import * as vscode from 'vscode';
import { getConfig, workspaceRoot } from '../config';
import {
  type CheckoutResult,
  type KanbanCard,
  checkoutCardBranch,
  loadKanbanCards,
} from '../integrations/cardBranch';
import { promptDirty } from '../integrations/dirtyPrompt';
import { type GitProvider, providerLabel } from '../integrations/gitRemote';
import type { Logger } from '../logger';

interface CardPick extends vscode.QuickPickItem {
  card: KanbanCard;
}

function showError(err: unknown): void {
  const message = err instanceof Error ? err.message : String(err);
  void vscode.window.showErrorMessage(`Pegg: ${message}`);
}

function cardLabel(card: KanbanCard): string {
  const number = card.issue?.number ?? card.issue?.iid;
  return number ? `#${number} ${card.title}` : card.title;
}

function cardDescription(card: KanbanCard): string {
  const parts: string[] = [];
  if (card.issue?.provider) parts.push(providerLabel(card.issue.provider as GitProvider));
  if (card.issue?.repo) parts.push(card.issue.repo);
  if (card.column) parts.push(card.column.replace(/_/g, ' '));
  return parts.join(' • ');
}

async function pickCard(cards: KanbanCard[]): Promise<KanbanCard | undefined> {
  const items: CardPick[] = cards.map((card) => ({
    label: cardLabel(card),
    description: cardDescription(card),
    detail: card.issue?.url,
    card,
  }));
  const picked = await vscode.window.showQuickPick(items, {
    title: 'Pegg: checkout a card branch',
    placeHolder: 'Pick a kanban card',
    matchOnDescription: true,
    matchOnDetail: true,
  });
  return picked?.card;
}

function summary(result: CheckoutResult): string {
  if (result.alreadyOnBranch) return `Already on ${result.branch}.`;
  const action = result.created ? 'Created and checked out' : 'Checked out';
  const pushed = result.pushed ? ` and pushed to ${result.remote}` : '';
  const stashed = result.stashed ? ' Local changes were stashed.' : '';
  const committed = result.committed ? ' Local changes were committed.' : '';
  return `${action} ${result.branch}${pushed}.${stashed}${committed}`;
}

async function runCheckout(card: KanbanCard | undefined, log: Logger): Promise<void> {
  const root = workspaceRoot();
  if (!root) {
    void vscode.window.showErrorMessage('Pegg: Open a folder first.');
    return;
  }

  const cfg = getConfig();
  let selected = card;
  if (!selected) {
    const cards = await loadKanbanCards(root);
    if (cards.length === 0) {
      void vscode.window.showInformationMessage(
        'Pegg: No kanban cards found. Start the kanban board to create .pegg/kanban/board.json.',
      );
      return;
    }
    selected = await pickCard(cards);
    if (!selected) return;
  }

  const target = selected;
  const result = await vscode.window.withProgress(
    {
      location: vscode.ProgressLocation.Notification,
      title: `Pegg: checking out ${target.title}`,
      cancellable: false,
    },
    (progress) =>
      checkoutCardBranch(target, {
        cwd: root,
        remote: cfg.gitRemote,
        providerOverride: cfg.gitProvider,
        branchTemplate: cfg.branchTemplate,
        autoPush: cfg.gitAutoPush,
        onProgress: (message) => progress.report({ message }),
        confirmDirty: promptDirty,
      }),
  );

  log.info(`Checked out card branch ${result.branch}`);
  void vscode.window.showInformationMessage(`Pegg: ${summary(result)}`);
}

export function registerKanbanCommands(log: Logger): vscode.Disposable[] {
  return [
    vscode.commands.registerCommand('pegg.kanban.checkoutBranch', async (card?: KanbanCard) => {
      try {
        await runCheckout(card, log);
      } catch (err) {
        log.warn('Could not checkout card branch', err);
        showError(err);
      }
    }),
  ];
}
