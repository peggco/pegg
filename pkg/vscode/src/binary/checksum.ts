import { createHash } from 'node:crypto';
import { createReadStream } from 'node:fs';
import { basename } from 'node:path';

export function parseChecksums(text: string): Map<string, string> {
  const out = new Map<string, string>();
  for (const raw of text.split(/\r?\n/)) {
    const line = raw.trim();
    if (!line) continue;
    const match = /^([0-9a-fA-F]{64})\s+\*?(.+)$/.exec(line);
    if (!match) continue;
    out.set(basename(match[2].trim()), match[1].toLowerCase());
  }
  return out;
}

export async function sha256File(path: string): Promise<string> {
  return new Promise((resolve, reject) => {
    const hash = createHash('sha256');
    const stream = createReadStream(path);
    stream.on('error', reject);
    stream.on('data', (chunk) => hash.update(chunk));
    stream.on('end', () => resolve(hash.digest('hex')));
  });
}

export async function verifyChecksum(
  archivePath: string,
  checksums: Map<string, string>,
  asset: string,
): Promise<void> {
  const expected = checksums.get(asset);
  if (!expected) throw new Error(`No checksum entry found for ${asset}`);
  const actual = await sha256File(archivePath);
  if (actual !== expected) {
    throw new Error(`Checksum mismatch for ${asset}\nExpected: ${expected}\nActual:   ${actual}`);
  }
}
