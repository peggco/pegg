import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import * as vscode from 'vscode';
import { workspaceRoot } from '../config';

const execFileAsync = promisify(execFile);
const MAX_DIFF = 400_000;

export interface GitDiffResult {
  text: string;
  files: string[];
  truncated: boolean;
}

export async function gitDiff(path?: string, staged = false): Promise<GitDiffResult> {
  const root = workspaceRoot();
  if (!root) throw new Error('Open a folder first.');
  const args = ['--no-pager', 'diff', '--no-color', '--src-prefix=a/', '--dst-prefix=b/'];
  if (staged) args.push('--cached');
  if (path) args.push('--', path);
  const { stdout } = await execFileAsync('git', args, {
    cwd: root,
    maxBuffer: 20 * 1024 * 1024,
    timeout: 30000,
  });
  const text = stdout.trim();
  const files = new Set<string>();
  for (const line of text.split('\n')) {
    const match = /^diff --git a\/(.+?) b\/(.+)$/.exec(line);
    if (match) files.add(match[2]);
  }
  return {
    text: text.length > MAX_DIFF ? `${text.slice(0, MAX_DIFF)}\n\n[diff truncated]` : text,
    files: [...files],
    truncated: text.length > MAX_DIFF,
  };
}

export async function isGitRepository(): Promise<boolean> {
  const root = workspaceRoot();
  if (!root) return false;
  try {
    await execFileAsync('git', ['rev-parse', '--is-inside-work-tree'], { cwd: root, timeout: 10000 });
    return true;
  } catch {
    return false;
  }
}

export async function currentBranch(): Promise<string | undefined> {
  const root = workspaceRoot();
  if (!root) return undefined;
  try {
    const { stdout } = await execFileAsync('git', ['rev-parse', '--abbrev-ref', 'HEAD'], {
      cwd: root,
      timeout: 10000,
    });
    return stdout.trim();
  } catch {
    return undefined;
  }
}

export function relativePath(path: string): string {
  return vscode.workspace.asRelativePath(vscode.Uri.file(path), false);
}
