import { afterAll, beforeAll, describe, expect, it } from 'vitest';
import { chmod, mkdir, mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { BinaryManager } from '../../src/binary/manager';
import { Logger } from '../../src/logger';
import { __setConfig, Uri } from '../fixtures/vscode';

describe.skipIf(process.platform === 'win32')('binary resolution diagnostics', () => {
  let root: string;
  let log: Logger;

  beforeAll(async () => {
    root = await mkdtemp(join(tmpdir(), 'pegg-resolution-diag-'));
    process.env.PEGG_TEST_WORKSPACE = join(root, 'workspace');
    await mkdir(join(root, 'workspace'), { recursive: true });
    log = new Logger();
  });

  afterAll(async () => {
    log?.dispose();
    if (root) await rm(root, { recursive: true, force: true });
  });

  function context(): any {
    return {
      extension: { packageJSON: { version: '0.1.9' } },
      globalStorageUri: Uri.file(join(root, 'storage')),
    };
  }

  it('runs the version probe inside the workspace', async () => {
    const workspace = join(root, 'workspace');
    const binary = join(root, 'cwd-check');
    await writeFile(
      binary,
      `#!/bin/sh\nif [ "$PWD" != "${workspace}" ]; then echo "unexpected cwd: $PWD" >&2; exit 1; fi\necho "pegg version 0.1.9"\n`,
    );
    await chmod(binary, 0o755);

    __setConfig('pegg.executablePath', binary);
    __setConfig('pegg.binary.autoDownload', false);

    const manager = new BinaryManager(context(), log);
    await expect(manager.ensure()).resolves.toBe(binary);
    expect(manager.currentVersion()).toBe('0.1.9');
  });

  it('reports why a binary fails the version check', async () => {
    const binary = join(root, 'broken');
    await writeFile(
      binary,
      '#!/bin/sh\necho "pegg: bootstrap: create project config dir: mkdir /.pegg: permission denied" >&2\nexit 1\n',
    );
    await chmod(binary, 0o755);

    __setConfig('pegg.executablePath', binary);

    const manager = new BinaryManager(context(), log);
    await expect(manager.ensure()).rejects.toThrow(/permission denied/);
  });

  it('lists probed candidates and the failure reason', async () => {
    const binary = join(root, 'broken-2');
    await writeFile(binary, '#!/bin/sh\necho "boom" >&2\nexit 2\n');
    await chmod(binary, 0o755);

    __setConfig('pegg.executablePath', binary);

    const manager = new BinaryManager(context(), log);
    const diagnostic = await manager.diagnose();
    expect(diagnostic.resolved).toBeUndefined();
    expect(diagnostic.error).toContain('boom');
    expect(diagnostic.candidates.length).toBeGreaterThan(0);
    expect(diagnostic.candidates[0].error).toContain('boom');
  });
});
