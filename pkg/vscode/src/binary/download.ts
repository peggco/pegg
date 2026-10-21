import { createWriteStream } from 'node:fs';
import { mkdir, readFile, rm } from 'node:fs/promises';
import { dirname } from 'node:path';
import { once } from 'node:events';
import * as tar from 'tar';
import extractZip from 'extract-zip';

export type Progress = (received: number, total: number) => void;

export async function downloadFile(url: string, dest: string, progress?: Progress): Promise<void> {
  await mkdir(dirname(dest), { recursive: true });
  const response = await fetch(url, {
    headers: { 'User-Agent': 'pegg-vscode', Accept: 'application/octet-stream,*/*' },
    redirect: 'follow',
    signal: AbortSignal.timeout(120_000),
  });
  if (!response.ok || !response.body) {
    throw new Error(`Download failed (HTTP ${response.status}) for ${url}`);
  }
  const total = Number(response.headers.get('content-length') ?? 0);
  const stream = createWriteStream(dest);
  const reader = response.body.getReader();
  let received = 0;
  try {
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      received += value.byteLength;
      if (!stream.write(Buffer.from(value))) await once(stream, 'drain');
      progress?.(received, total);
    }
    stream.end();
    await once(stream, 'finish');
  } catch (err) {
    stream.destroy();
    await rm(dest, { force: true });
    throw err;
  }
}

export async function downloadText(url: string): Promise<string> {
  const response = await fetch(url, {
    headers: { 'User-Agent': 'pegg-vscode' },
    redirect: 'follow',
    signal: AbortSignal.timeout(30_000),
  });
  if (!response.ok) throw new Error(`Download failed (HTTP ${response.status}) for ${url}`);
  return response.text();
}

export async function extractArchive(archive: string, dest: string, asset: string): Promise<void> {
  await mkdir(dest, { recursive: true });
  if (asset.endsWith('.zip')) {
    await extractZip(archive, { dir: dest });
    return;
  }
  await tar.x({ file: archive, cwd: dest });
}

export async function readBytes(path: string): Promise<Buffer> {
  return readFile(path);
}
