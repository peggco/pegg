import { afterAll, beforeAll, describe, expect, it } from 'vitest';
import { chmod, mkdir, mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { BinaryManager } from '../../src/binary/manager';
import { Logger } from '../../src/logger';
import { __setConfig, Uri } from '../fixtures/vscode';

describe.skipIf(process.platform === 'win32')('binary manager resolution', () => {
  let root: string;
  let log: Logger;

  beforeAll(async () => {
    root = await mkdtemp(join(tmpdir(), 'pegg-resolution-test-'));
    process.env.PEGG_TEST_WORKSPACE = join(root, 'workspace');
    await mkdir(join(root, 'workspace', 'bin'), { recursive: true });
    const workspaceBinary = join(root, 'workspace', 'bin', 'pegg');
    await writeFile(workspaceBinary, '#!/bin/sh\necho "pegg version 0.1.9"\n');
    await chmod(workspaceBinary, 0o755);

    const cached = join(root, 'storage', 'bin', 'v0.1.9');
    await mkdir(cached, { recursive: true });
    const cachedBinary = join(cached, 'pegg');
    await writeFile(cachedBinary, '#!/bin/sh\necho "pegg version 0.1.9"\n');
    await chmod(cachedBinary, 0o755);

    __setConfig('pegg.executablePath', '');
    __setConfig('pegg.binary.autoDownload', false);
    __setConfig('pegg.binary.allowWorkspaceBinary', true);
    __setConfig('pegg.binary.version', 'v0.1.9');
    log = new Logger();
  });

  afterAll(async () => {
    log?.dispose();
    if (root) await rm(root, { recursive: true, force: true });
  });

  it('prefers the trusted workspace binary over a cached download', async () => {
    const context: any = {
      extension: { packageJSON: { version: '0.1.9' } },
      globalStorageUri: Uri.file(join(root, 'storage')),
    };
    const manager = new BinaryManager(context, log);
    const binary = await manager.ensure();
    expect(binary).toContain(join('workspace', 'bin', 'pegg'));
    expect(manager.currentVersion()).toBe('0.1.9');
  });
});
