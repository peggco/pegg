import { execFile } from 'node:child_process';
import { mkdtemp, mkdir, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { promisify } from 'node:util';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import {
  CardCheckoutError,
  cardBranch,
  checkoutCardBranch,
  loadKanbanCards,
} from '../../src/integrations/cardBranch';

const execFileAsync = promisify(execFile);

async function git(cwd: string, args: string[]): Promise<string> {
  const { stdout } = await execFileAsync('git', args, {
    cwd,
    env: {
      ...process.env,
      GIT_AUTHOR_NAME: 'Pegg Test',
      GIT_AUTHOR_EMAIL: 'test@example.com',
      GIT_COMMITTER_NAME: 'Pegg Test',
      GIT_COMMITTER_EMAIL: 'test@example.com',
      GIT_TERMINAL_PROMPT: '0',
    },
  });
  return stdout.trim();
}

let root: string;
let repo: string;
let remote: string;

beforeEach(async () => {
  root = await mkdtemp(join(tmpdir(), 'pegg-card-'));
  remote = join(root, 'remote.git');
  repo = join(root, 'repo');
  await mkdir(repo);
  await git(root, ['init', '--bare', remote]);
  await git(remote, ['symbolic-ref', 'HEAD', 'refs/heads/main']);
  await git(repo, ['init', '-b', 'main']);
  await writeFile(join(repo, 'README.md'), 'hello\n');
  await git(repo, ['add', '.']);
  await git(repo, ['commit', '-m', 'initial']);
  await git(repo, ['remote', 'add', 'origin', remote]);
  await git(repo, ['push', '-u', 'origin', 'main']);
});

afterEach(async () => {
  await rm(root, { recursive: true, force: true });
});

function opts(overrides: Record<string, unknown> = {}) {
  return {
    cwd: repo,
    providerOverride: 'github',
    branchTemplate: '{number}-{slug}',
    ...overrides,
  };
}

describe('cardBranch', () => {
  it('prefers a branch supplied by the card context', () => {
    expect(
      cardBranch(
        { title: 'Fix login', issue: { number: 42 }, context: { branch: 'pegg/fix-login' } },
        opts() as never,
      ),
    ).toBe('pegg/fix-login');
  });

  it('strips refs/heads and remote prefixes from explicit branches', () => {
    expect(cardBranch({ title: 'Fix login', branch: 'refs/heads/feature/x' }, opts() as never)).toBe(
      'feature/x',
    );
    expect(cardBranch({ title: 'Fix login', branch: 'origin/feature/x' }, opts() as never)).toBe(
      'feature/x',
    );
  });

  it('renders the configured template from the issue number and title', () => {
    expect(
      cardBranch({ title: '[BUG] Fix login redirect', issue: { number: 42 } }, opts() as never),
    ).toBe('42-fix-login-redirect');
    expect(
      cardBranch(
        { title: 'Fix login', issue: { iid: 9 } },
        opts({ branchTemplate: 'issue/{number}-{slug}' }) as never,
      ),
    ).toBe('issue/9-fix-login');
  });
});

describe('loadKanbanCards', () => {
  it('returns an empty list when the board is missing', async () => {
    expect(await loadKanbanCards(root)).toEqual([]);
  });

  it('reads and sorts cards from board.json', async () => {
    await mkdir(join(root, '.pegg', 'kanban'), { recursive: true });
    await writeFile(
      join(root, '.pegg', 'kanban', 'board.json'),
      JSON.stringify({
        cards: [
          { id: 'b', title: 'Second', issue: { number: 2 } },
          { id: 'a', title: 'First', issue: { number: 1 }, context: { branch: 'pegg/first' } },
          { id: 'c', title: 'No issue' },
        ],
      }),
    );
    const cards = await loadKanbanCards(root);
    expect(cards.map((c) => c.title)).toEqual(['First', 'Second', 'No issue']);
    expect(cards[0].context?.branch).toBe('pegg/first');
  });

  it('reports malformed boards', async () => {
    await mkdir(join(root, '.pegg', 'kanban'), { recursive: true });
    await writeFile(join(root, '.pegg', 'kanban', 'board.json'), '{not json');
    await expect(loadKanbanCards(root)).rejects.toBeInstanceOf(CardCheckoutError);
  });
});

describe('checkoutCardBranch', () => {
  it('checks out an existing remote branch with tracking', async () => {
    await git(repo, ['branch', '42-remote']);
    await git(repo, ['push', 'origin', '42-remote']);
    await git(repo, ['branch', '-D', '42-remote']);

    const result = await checkoutCardBranch({ title: 'Remote', issue: { number: 42 } }, opts());

    expect(result).toMatchObject({ branch: '42-remote', created: false, provider: 'github' });
    expect(await git(repo, ['rev-parse', '--abbrev-ref', 'HEAD'])).toBe('42-remote');
    expect(await git(repo, ['rev-parse', '--abbrev-ref', '42-remote@{upstream}'])).toBe(
      'origin/42-remote',
    );
  });

  it('checks out a local-only branch without touching the remote', async () => {
    await git(repo, ['branch', '43-local']);

    const result = await checkoutCardBranch({ title: 'Local', issue: { number: 43 } }, opts());

    expect(result.created).toBe(false);
    expect(await git(repo, ['rev-parse', '--abbrev-ref', 'HEAD'])).toBe('43-local');
    await expect(git(remote, ['rev-parse', '--verify', 'refs/heads/43-local'])).rejects.toBeTruthy();
  });

  it('creates a missing branch from the remote default branch', async () => {
    const result = await checkoutCardBranch(
      { title: 'Brand new thing', issue: { number: 44 } },
      opts(),
    );

    expect(result).toMatchObject({ branch: '44-brand-new-thing', base: 'main', created: true });
    expect(await git(repo, ['rev-parse', '--abbrev-ref', 'HEAD'])).toBe('44-brand-new-thing');
    expect(await git(repo, ['rev-parse', 'HEAD'])).toBe(await git(repo, ['rev-parse', 'origin/main']));
  });

  it('pushes a newly created branch when auto-push is enabled', async () => {
    const result = await checkoutCardBranch(
      { title: 'Push me', issue: { number: 45 } },
      opts({ autoPush: true }),
    );

    expect(result).toMatchObject({ created: true, pushed: true });
    expect(await git(remote, ['rev-parse', '--verify', 'refs/heads/45-push-me'])).toBeTruthy();
  });

  it('is a no-op when already on the branch', async () => {
    const result = await checkoutCardBranch(
      { title: 'Already here', issue: { number: 46 }, context: { branch: 'main' } },
      opts(),
    );

    expect(result.alreadyOnBranch).toBe(true);
    expect(await git(repo, ['rev-parse', '--abbrev-ref', 'HEAD'])).toBe('main');
  });

  it('refuses to modify a dirty worktree when the user cancels', async () => {
    await writeFile(join(repo, 'README.md'), 'changed\n');
    const confirmDirty = async () => 'cancel' as const;

    await expect(
      checkoutCardBranch({ title: 'Dirty', issue: { number: 47 } }, opts({ confirmDirty })),
    ).rejects.toThrow(/uncommitted changes/i);
    expect(await git(repo, ['status', '--porcelain'])).not.toBe('');
  });

  it('stashes a dirty worktree when asked', async () => {
    await writeFile(join(repo, 'README.md'), 'changed\n');
    const result = await checkoutCardBranch(
      { title: 'Dirty', issue: { number: 48 } },
      opts({ confirmDirty: async () => 'stash' }),
    );

    expect(result.stashed).toBe(true);
    expect(await git(repo, ['status', '--porcelain'])).toBe('');
    expect(await git(repo, ['stash', 'list'])).toContain('pegg: changes before checking out 48-dirty');
  });

  it('commits a dirty worktree when asked', async () => {
    await writeFile(join(repo, 'README.md'), 'changed\n');
    const result = await checkoutCardBranch(
      { title: 'Dirty', issue: { number: 49 } },
      opts({ confirmDirty: async () => 'commit' }),
    );

    expect(result.committed).toBe(true);
    expect(await git(repo, ['status', '--porcelain'])).toBe('');
    expect(await git(repo, ['log', '-1', '--pretty=%s'])).toContain('wip: changes before checking out 49-dirty');
  });

  it('rejects an unparseable remote without a provider override', async () => {
    await expect(
      checkoutCardBranch(
        { title: 'No provider', issue: { number: 50 } },
        opts({ providerOverride: '' }),
      ),
    ).rejects.toThrow(/Could not detect the hosting provider/i);
  });

  it('reports a missing remote actionably', async () => {
    await expect(
      checkoutCardBranch(
        { title: 'No remote', issue: { number: 51 } },
        opts({ remote: 'upstream' }),
      ),
    ).rejects.toThrow(/No git remote named 'upstream'/);
  });

  it('rejects invalid branch names', async () => {
    await expect(
      checkoutCardBranch(
        { title: 'Bad name', issue: { number: 52 }, context: { branch: 'bad..name' } },
        opts(),
      ),
    ).rejects.toBeInstanceOf(CardCheckoutError);
  });
});
