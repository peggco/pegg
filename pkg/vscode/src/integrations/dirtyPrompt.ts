import * as vscode from 'vscode';
import type { DirtyChoice } from './cardBranch';

export async function promptDirty(files: string[]): Promise<DirtyChoice> {
  const choice = await vscode.window.showWarningMessage(
    `The working tree has ${files.length} uncommitted change${files.length === 1 ? '' : 's'}.`,
    {
      modal: true,
      detail: 'Stash keeps the changes for later. Commit creates a checkpoint commit first. Pegg never discards changes.',
    },
    'Stash',
    'Commit',
    'Cancel',
  );
  if (choice === 'Stash') return 'stash';
  if (choice === 'Commit') return 'commit';
  return 'cancel';
}
