import { describe, expect, it } from 'vitest';
import { mkdir, mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { extraSearchPaths, profileSearchPaths, searchDirs } from '../../src/binary/paths';
import { detectTarget } from '../../src/binary/platform';

describe('binary search paths', () => {
  it('includes the common install locations', () => {
    const dirs = extraSearchPaths('darwin', '/home/tester');
    expect(dirs).toContain('/opt/homebrew/bin');
    expect(dirs).toContain('/usr/local/bin');
    expect(dirs).toContain('/home/tester/.local/bin');
    expect(dirs).toContain('/home/tester/.pegg/bin');
  });

  it('reads PATH entries from shell profiles', async () => {
    const home = await mkdtemp(join(tmpdir(), 'pegg-home-'));
    try {
      await writeFile(
        join(home, '.zshrc'),
        ['export PATH="$HOME/.local/bin:$PATH"', 'export PATH="/opt/tools/bin:$PATH"'].join('\n'),
      );
      const dirs = await profileSearchPaths(home);
      expect(dirs).toContain('/home/tester/.local/bin'.replace('/home/tester', home));
      expect(dirs).toContain('/opt/tools/bin');
      expect(dirs.some((dir) => dir.includes('$PATH'))).toBe(false);
    } finally {
      await rm(home, { recursive: true, force: true });
    }
  });

  it('combines PATH, well known directories and profile directories', async () => {
    const home = await mkdtemp(join(tmpdir(), 'pegg-home-'));
    try {
      await mkdir(join(home, 'bin'), { recursive: true });
      await writeFile(join(home, '.profile'), 'PATH="$HOME/bin:$PATH"\n');
      const dirs = await searchDirs('linux', home);
      expect(dirs).toContain(join(home, 'bin'));
      expect(dirs).toContain('/usr/local/bin');
      expect(new Set(dirs).size).toBe(dirs.length);
    } finally {
      await rm(home, { recursive: true, force: true });
    }
  });
});

describe('release asset urls', () => {
  it('uses the latest download endpoint for the latest tag', async () => {
    const requested: string[] = [];
    const originalFetch = globalThis.fetch;
    globalThis.fetch = (async (url: string) => {
      requested.push(String(url));
      return {
        ok: false,
        status: 404,
        headers: { get: () => null },
        body: null,
      } as unknown as Response;
    }) as typeof fetch;

    try {
      const { BinaryManager } = await import('../../src/binary/manager');
      const { __setConfig, Uri } = await import('../fixtures/vscode');
      __setConfig('pegg.executablePath', '');
      __setConfig('pegg.binary.autoDownload', true);
      __setConfig('pegg.binary.version', 'latest');

      const context: any = {
        extension: { packageJSON: { version: '0.1.9' } },
        globalStorageUri: Uri.file(join(tmpdir(), 'pegg-latest-test')),
      };
      const manager = new BinaryManager(context, { info() {}, debug() {}, warn() {}, error() {} } as any);
      await expect(manager.ensure({ force: true })).rejects.toThrow();

      const target = detectTarget();
      expect(requested[0]).toBe(
        `https://github.com/peggco/pegg/releases/latest/download/${target.asset}`,
      );
    } finally {
      globalThis.fetch = originalFetch;
    }
  });
});
