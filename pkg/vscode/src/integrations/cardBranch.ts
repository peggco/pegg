import { execFile } from 'node:child_process';
import { readFile } from 'node:fs/promises';
import * as path from 'node:path';
import { promisify } from 'node:util';
import { branchNameFor } from './branchName';
import { detectProvider, providerLabel, type GitProvider, parseRemoteUrl } from './gitRemote';

const execFileAsync = promisify(execFile);
const GIT_TIMEOUT = 60_000;

export interface CardIssue {
  provider?: string;
  repo?: string;
  number?: number;
  iid?: number;
  url?: string;
}

export interface CardBranchContext {
  branch?: string;
  base?: string;
}

export interface KanbanCard {
  id?: string;
  title: string;
  column?: string;
  issue?: CardIssue;
  context?: CardBranchContext;
  branch?: string;
  base?: string;
}

export type DirtyChoice = 'stash' | 'commit' | 'cancel';

export interface CheckoutOptions {
  cwd: string;
  remote?: string;
  providerOverride?: string;
  branchTemplate: string;
  autoPush?: boolean;
  confirmDirty?: (files: string[]) => Promise<DirtyChoice>;
  onProgress?: (message: string) => void;
}

export interface CheckoutResult {
  branch: string;
  provider: GitProvider;
  remote: string;
  base?: string;
  created: boolean;
  pushed: boolean;
  stashed: boolean;
  committed: boolean;
  alreadyOnBranch: boolean;
}

export class CardCheckoutError extends Error {
  constructor(
    message: string,
    readonly exitCode?: number,
  ) {
    super(message);
    this.name = 'CardCheckoutError';
  }
}

function firstLine(text: string): string {
  return (
    text
      .split('\n')
      .map((line) => line.trim())
      .filter(Boolean)
      .slice(-3)
      .join(' ') || ''
  );
}

function actionable(message: string, provider: GitProvider): string {
  if (/could not resolve host|unable to access|connection timed out|failed to connect|network is unreachable/i.test(message)) {
    return 'Could not reach the git remote. Check your network connection and retry.';
  }
  if (/authentication failed|could not read username|permission denied \(publickey\)|invalid username or password|terminal prompts disabled|returned error: 40[13]/i.test(message)) {
    return `The ${providerLabel(provider)} remote rejected authentication. Sign in and retry.`;
  }
  if (/would be overwritten by checkout|local changes/i.test(message)) {
    return 'Local changes would be overwritten by the checkout. Stash or commit them first.';
  }
  if (/already exists/i.test(message)) {
    return 'A local branch with that name already exists. Pick another branch name.';
  }
  if (/not a git repository/i.test(message)) {
    return 'The workspace is not a git repository.';
  }
  return message || 'git command failed.';
}

async function git(cwd: string, args: string[], provider: GitProvider): Promise<string> {
  try {
    const { stdout } = await execFileAsync('git', args, {
      cwd,
      timeout: GIT_TIMEOUT,
      maxBuffer: 10 * 1024 * 1024,
      env: { ...process.env, GIT_TERMINAL_PROMPT: '0' },
    });
    return stdout.trim();
  } catch (err) {
    const error = err as { code?: number; stderr?: string; stdout?: string; message?: string };
    const detail = firstLine(error.stderr || error.stdout || error.message || '');
    throw new CardCheckoutError(actionable(detail, provider), error.code);
  }
}

function normalizeBranch(branch: string, remote: string): string {
  const withoutRefs = branch.trim().replace(/^refs\/heads\//, '');
  return withoutRefs.startsWith(`${remote}/`) ? withoutRefs.slice(remote.length + 1) : withoutRefs;
}

export function cardBranch(card: KanbanCard, opts: CheckoutOptions): string {
  const remote = opts.remote?.trim() || 'origin';
  const explicit = card.context?.branch ?? card.branch;
  if (explicit?.trim()) return normalizeBranch(explicit, remote);
  const issue = card.issue ?? {};
  return branchNameFor(opts.branchTemplate, {
    number: issue.number ?? issue.iid,
    title: card.title,
    id: card.id,
    provider: issue.provider,
  });
}

export async function loadKanbanCards(root: string): Promise<KanbanCard[]> {
  const file = path.join(root, '.pegg', 'kanban', 'board.json');
  let raw: string;
  try {
    raw = await readFile(file, 'utf8');
  } catch {
    return [];
  }
  let data: unknown;
  try {
    data = JSON.parse(raw);
  } catch {
    throw new CardCheckoutError(`Could not parse ${file}. Fix the board file and retry.`);
  }
  const cards: unknown[] = [];
  if (data && typeof data === 'object') {
    const board = data as { cards?: unknown; columns?: unknown };
    if (Array.isArray(board.cards)) cards.push(...board.cards);
    if (Array.isArray(board.columns)) {
      for (const column of board.columns) {
        if (column && typeof column === 'object' && Array.isArray((column as { cards?: unknown }).cards)) {
          cards.push(...((column as { cards: unknown[] }).cards ?? []));
        }
      }
    }
  }
  const parsed: KanbanCard[] = [];
  for (const entry of cards) {
    if (!entry || typeof entry !== 'object') continue;
    const card = entry as Record<string, unknown>;
    if (typeof card.title !== 'string' || !card.title.trim()) continue;
    const issue = card.issue && typeof card.issue === 'object' ? (card.issue as CardIssue) : undefined;
    const context = card.context && typeof card.context === 'object' ? (card.context as CardBranchContext) : undefined;
    parsed.push({
      id: typeof card.id === 'string' ? card.id : undefined,
      title: card.title,
      column: typeof card.column === 'string' ? card.column : undefined,
      issue,
      context,
      branch: typeof card.branch === 'string' ? card.branch : undefined,
      base: typeof card.base === 'string' ? card.base : undefined,
    });
  }
  parsed.sort((a, b) => {
    const an = a.issue?.number ?? a.issue?.iid ?? Number.MAX_SAFE_INTEGER;
    const bn = b.issue?.number ?? b.issue?.iid ?? Number.MAX_SAFE_INTEGER;
    if (an !== bn) return an - bn;
    return a.title.localeCompare(b.title);
  });
  return parsed;
}

async function refExists(cwd: string, ref: string, provider: GitProvider): Promise<boolean> {
  try {
    await git(cwd, ['show-ref', '--verify', '--quiet', ref], provider);
    return true;
  } catch (err) {
    if (err instanceof CardCheckoutError && (err.exitCode === 1 || err.exitCode === 128)) return false;
    throw err;
  }
}

async function remoteBranchExists(
  cwd: string,
  remote: string,
  branch: string,
  provider: GitProvider,
): Promise<boolean> {
  try {
    await git(cwd, ['ls-remote', '--exit-code', '--heads', remote, `refs/heads/${branch}`], provider);
    return true;
  } catch (err) {
    if (err instanceof CardCheckoutError && err.exitCode === 2) return false;
    throw err;
  }
}

async function remoteDefaultBranch(
  cwd: string,
  remote: string,
  provider: GitProvider,
): Promise<string> {
  try {
    const ref = await git(cwd, ['symbolic-ref', '--short', `refs/remotes/${remote}/HEAD`], provider);
    if (ref) return normalizeBranch(ref, remote);
  } catch {
  }
  try {
    const out = await git(cwd, ['ls-remote', '--symref', remote, 'HEAD'], provider);
    const match = /^ref:\s+refs\/heads\/(\S+)\s+HEAD/m.exec(out);
    if (match?.[1]) return match[1];
  } catch {
  }
  return 'main';
}

async function currentBranch(cwd: string, provider: GitProvider): Promise<string> {
  try {
    return await git(cwd, ['rev-parse', '--abbrev-ref', 'HEAD'], provider);
  } catch {
    return '';
  }
}

async function dirtyFiles(cwd: string, provider: GitProvider): Promise<string[]> {
  const out = await git(cwd, ['status', '--porcelain'], provider);
  return out
    .split('\n')
    .map((line) => line.slice(3).trim())
    .filter(Boolean);
}

export async function checkoutCardBranch(
  card: KanbanCard,
  opts: CheckoutOptions,
): Promise<CheckoutResult> {
  const hint = (message: string) => opts.onProgress?.(message);
  const remote = opts.remote?.trim() || 'origin';
  const cwd = opts.cwd;

  const inside = await git(cwd, ['rev-parse', '--is-inside-work-tree'], 'unknown');
  if (inside !== 'true') throw new CardCheckoutError('The workspace is not a git repository.');

  let remoteUrl: string;
  try {
    remoteUrl = await git(cwd, ['remote', 'get-url', remote], 'unknown');
  } catch {
    throw new CardCheckoutError(
      `No git remote named '${remote}' in this repository. Set the 'pegg.git.remote' option or add the remote.`,
    );
  }
  const parsed = parseRemoteUrl(remoteUrl, opts.providerOverride);
  const provider = parsed?.provider ?? detectProvider('', opts.providerOverride);
  if (provider === 'unknown') {
    const host = parsed?.host ? ` (${parsed.host})` : '';
    throw new CardCheckoutError(
      `Could not detect the hosting provider${host}. Set 'pegg.git.provider' to github, bitbucket, or gitlab.`,
      undefined,
    );
  }

  const branch = cardBranch(card, opts);
  if (!branch) throw new CardCheckoutError('Could not derive a branch name for this card.');
  hint(`Checking branch ${branch}`);
  await git(cwd, ['check-ref-format', '--branch', branch], provider);

  const current = await currentBranch(cwd, provider);
  if (current === branch) {
    hint(`Already on ${branch}`);
    return {
      branch,
      provider,
      remote,
      base: card.context?.base ?? card.base,
      created: false,
      pushed: false,
      stashed: false,
      committed: false,
      alreadyOnBranch: true,
    };
  }

  let stashed = false;
  let committed = false;
  const dirty = await dirtyFiles(cwd, provider);
  if (dirty.length > 0) {
    const choice = opts.confirmDirty ? await opts.confirmDirty(dirty) : 'cancel';
    if (choice === 'cancel') {
      throw new CardCheckoutError(
        'Checkout cancelled because the working tree has uncommitted changes.',
      );
    }
    if (choice === 'stash') {
      hint('Stashing local changes');
      await git(cwd, ['stash', 'push', '--include-untracked', '-m', `pegg: changes before checking out ${branch}`], provider);
      stashed = true;
    } else {
      hint('Committing local changes');
      await git(cwd, ['add', '-A'], provider);
      await git(cwd, ['commit', '-m', `wip: changes before checking out ${branch}`], provider);
      committed = true;
    }
  }

  if (await refExists(cwd, `refs/heads/${branch}`, provider)) {
    hint(`Checking out ${branch}`);
    await git(cwd, ['checkout', branch], provider);
    return {
      branch,
      provider,
      remote,
      base: card.context?.base ?? card.base,
      created: false,
      pushed: false,
      stashed,
      committed,
      alreadyOnBranch: false,
    };
  }

  const onRemote = await remoteBranchExists(cwd, remote, branch, provider);
  if (onRemote) {
    hint(`Fetching ${remote}/${branch}`);
    await git(cwd, ['fetch', remote, `refs/heads/${branch}:refs/remotes/${remote}/${branch}`], provider);
    hint(`Checking out ${branch}`);
    await git(cwd, ['checkout', '-b', branch, '--track', `${remote}/${branch}`], provider);
    return {
      branch,
      provider,
      remote,
      base: card.context?.base ?? card.base,
      created: false,
      pushed: false,
      stashed,
      committed,
      alreadyOnBranch: false,
    };
  }

  const base = await remoteDefaultBranch(cwd, remote, provider);
  hint(`Creating ${branch} from ${committed ? 'HEAD' : `${remote}/${base}`}`);
  if (!committed && !(await refExists(cwd, `refs/remotes/${remote}/${base}`, provider))) {
    await git(cwd, ['fetch', remote, `refs/heads/${base}:refs/remotes/${remote}/${base}`], provider);
  }
  await git(cwd, ['checkout', '-b', branch, committed ? 'HEAD' : `${remote}/${base}`], provider);

  let pushed = false;
  if (opts.autoPush) {
    hint(`Pushing ${branch} to ${remote}`);
    await git(cwd, ['push', '-u', remote, branch], provider);
    pushed = true;
  }
  return {
    branch,
    provider,
    remote,
    base,
    created: true,
    pushed,
    stashed,
    committed,
    alreadyOnBranch: false,
  };
}
