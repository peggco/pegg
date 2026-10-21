import { afterAll, describe, expect, it } from 'vitest';
import { mkdtemp, rm, stat } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { BinaryManager } from '../../src/binary/manager';
import { Logger } from '../../src/logger';
import { __setConfig, Uri } from '../fixtures/vscode';

describe.skipIf(!process.env.PEGG_TEST_DOWNLOAD)('binary manager download', () => {
  let cwd: string;
  let log: Logger;

  afterAll(async () => {
    log?.dispose();
    if (cwd) await rm(cwd, { recursive: true, force: true });
  });

  it('downloads, verifies and installs the release binary', async () => {
    cwd = await mkdtemp(join(tmpdir(), 'pegg-binary-test-'));
    __setConfig('pegg.executablePath', '');
    __setConfig('pegg.binary.autoDownload', true);
    __setConfig('pegg.binary.allowWorkspaceBinary', false);
    __setConfig('pegg.binary.version', 'v0.1.9');
    log = new Logger();
    const context: any = {
      extension: { packageJSON: { version: '0.1.9' } },
      globalStorageUri: Uri.file(join(cwd, 'storage')),
    };
    const manager = new BinaryManager(context, log);
    const binary = await manager.ensure({ force: true });
    expect((await stat(binary)).isFile()).toBe(true);
    expect(manager.currentVersion()).toBe('0.1.9');
  }, 180_000);
});
