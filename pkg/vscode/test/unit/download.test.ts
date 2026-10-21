import { afterAll, beforeAll, describe, expect, it } from 'vitest';
import { createServer, Server } from 'node:http';
import { readFileSync } from 'node:fs';
import { mkdir, mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import * as tar from 'tar';
import { downloadFile, extractArchive } from '../../src/binary/download';
import { parseChecksums, sha256File, verifyChecksum } from '../../src/binary/checksum';

describe('binary download primitives', () => {
  let server: Server;
  let base: string;
  let root: string;
  let archive: string;
  let checksums: string;
  const asset = 'pegg_linux_amd64.tar.gz';

  beforeAll(async () => {
    root = await mkdtemp(join(tmpdir(), 'pegg-download-test-'));
    const stage = join(root, 'stage');
    await mkdir(stage, { recursive: true });
    await writeFile(join(stage, 'pegg'), '#!/bin/sh\necho "pegg version 9.9.9"\n', { mode: 0o755 });
    archive = join(root, asset);
    await tar.c({ gzip: true, file: archive, cwd: stage }, ['pegg']);
    checksums = `${await sha256File(archive)}  ${asset}\n`;

    server = createServer((request, response) => {
      if (request.url?.endsWith('checksums.txt')) {
        response.writeHead(200, { 'content-type': 'text/plain' });
        response.end(checksums);
        return;
      }
      response.writeHead(200, { 'content-type': 'application/octet-stream' });
      response.end(readFileSync(archive));
    });
    await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
    const address = server.address();
    const port = typeof address === 'object' && address ? address.port : 0;
    base = `http://127.0.0.1:${port}`;
  });

  afterAll(async () => {
    await new Promise<void>((resolve) => server.close(() => resolve()));
    await rm(root, { recursive: true, force: true });
  });

  it('downloads, verifies and extracts an archive', async () => {
    const downloaded = join(root, 'download', asset);
    await downloadFile(`${base}/${asset}`, downloaded);
    const checksumsPath = join(root, 'download', 'checksums.txt');
    await downloadFile(`${base}/checksums.txt`, checksumsPath);
    const parsed = parseChecksums(await readFile(checksumsPath, 'utf8'));
    await verifyChecksum(downloaded, parsed, asset);
    const out = join(root, 'extract');
    await extractArchive(downloaded, out, asset);
    expect(await readFile(join(out, 'pegg'), 'utf8')).toContain('pegg version');
  });

  it('rejects a bad checksum', async () => {
    const downloaded = join(root, 'download2', asset);
    await downloadFile(`${base}/${asset}`, downloaded);
    const parsed = new Map([[asset, '0'.repeat(64)]]);
    await expect(verifyChecksum(downloaded, parsed, asset)).rejects.toThrow(/Checksum mismatch/);
  });

  it('rejects a missing checksum', async () => {
    await expect(verifyChecksum(join(root, 'nope'), new Map(), asset)).rejects.toThrow(/No checksum/);
  });
});
