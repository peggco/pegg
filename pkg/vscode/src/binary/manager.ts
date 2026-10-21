import * as vscode from 'vscode';
import { execFile } from 'node:child_process';
import { constants } from 'node:fs';
import { access, chmod, copyFile, mkdir, readdir, readFile, rename, rm, stat, writeFile } from 'node:fs/promises';
import { homedir } from 'node:os';
import { delimiter, join, resolve } from 'node:path';
import { promisify } from 'node:util';
import { getConfig, workspaceRoot } from '../config';
import { Logger } from '../logger';
import { parseChecksums, verifyChecksum } from './checksum';
import { downloadFile, extractArchive } from './download';
import { detectTarget } from './platform';

const execFileAsync = promisify(execFile);

export interface EnsureOptions {
  force?: boolean;
  progress?: (message: string, increment: number) => void;
}

export class BinaryManager {
  private cached: string | undefined;
  private version: string | undefined;

  constructor(
    private readonly context: vscode.ExtensionContext,
    private readonly log: Logger,
  ) {}

  async ensure(options: EnsureOptions = {}): Promise<string> {
    const cfg = getConfig();
    if (options.force) this.cached = undefined;

    if (cfg.executablePath) {
      const explicit = await this.verify(cfg.executablePath);
      if (!explicit) {
        throw new Error(`pegg.executablePath does not point to a working binary: ${cfg.executablePath}`);
      }
      return this.use(explicit);
    }

    if (!options.force) {
      const candidate = await this.findExisting();
      if (candidate) return this.use(candidate);
    }

    if (!cfg.autoDownload) {
      throw new Error(
        'Pegg binary not found. Install Pegg, set pegg.executablePath, or enable pegg.binary.autoDownload.',
      );
    }

    return this.download(options.progress);
  }

  currentVersion(): string | undefined {
    return this.version;
  }

  private async use(path: string): Promise<string> {
    this.cached = path;
    this.version = await this.readVersion(path);
    return path;
  }

  private async findExisting(): Promise<string | undefined> {
    if (this.cached) {
      const ok = await this.verify(this.cached);
      if (ok) return ok;
      this.cached = undefined;
    }

    const target = detectTarget();
    const root = workspaceRoot();
    const candidates: string[] = [];
    if (root && getConfig().allowWorkspaceBinary) candidates.push(join(root, 'bin', target.binary));
    for (const dir of (process.env.PATH ?? '').split(delimiter)) {
      if (dir) candidates.push(join(dir, target.binary));
    }
    candidates.push(this.cachedPath(await this.resolveTag(false)));
    for (const candidate of candidates) {
      const ok = await this.verify(candidate);
      if (ok) return ok;
    }
    return undefined;
  }

  private async verify(path: string): Promise<string | undefined> {
    if (!path) return undefined;
    try {
      if (process.platform !== 'win32') await access(path, constants.X_OK);
      else await access(path, constants.F_OK);
      const info = await stat(path);
      if (!info.isFile()) return undefined;
      const version = await this.readVersion(path);
      return version ? path : undefined;
    } catch {
      return undefined;
    }
  }

  private async readVersion(path: string): Promise<string | undefined> {
    try {
      const { stdout } = await execFileAsync(path, ['version'], { timeout: 15000 });
      const match = /pegg version\s+([^\s]+)/i.exec(stdout);
      return match?.[1] ?? undefined;
    } catch {
      return undefined;
    }
  }

  private async download(progress?: (message: string, increment: number) => void): Promise<string> {
    const cfg = getConfig();
    const tag = await this.resolveTag(true);
    const target = detectTarget();
    const dir = this.cachedDir(tag);
    const finalPath = join(dir, target.binary);

    const existing = await this.verify(finalPath);
    if (existing) return this.use(finalPath);

    progress?.('Resolving Pegg release…', 0);
    const base = (process.env.PEGG_DOWNLOAD_BASE_URL || cfg.downloadBaseUrl).replace(/\/+$/, '');
    const archive = join(dir, target.asset);
    const checksumsPath = join(dir, 'checksums.txt');
    await mkdir(dir, { recursive: true });
    await rm(finalPath, { force: true });

    try {
      progress?.(`Downloading ${target.asset}…`, 5);
      await downloadFile(`${base}/${tag}/${target.asset}`, archive, (received, total) => {
        if (total > 0) {
          const pct = Math.min(70, 5 + Math.round((received / total) * 65));
          progress?.(`Downloading ${target.asset}… ${Math.round((received / total) * 100)}%`, pct);
        }
      });

      progress?.('Downloading checksums…', 72);
      await downloadFile(`${base}/${tag}/checksums.txt`, checksumsPath);

      progress?.('Verifying checksum…', 78);
      const checksums = parseChecksums(await readFile(checksumsPath, 'utf8'));
      await verifyChecksum(archive, checksums, target.asset);

      progress?.('Extracting Pegg…', 85);
      const extractDir = join(dir, 'extract');
      await rm(extractDir, { recursive: true, force: true });
      await extractArchive(archive, extractDir, target.asset);
      const binary = await locateBinary(extractDir, target.binary);
      if (!binary) throw new Error(`The ${target.asset} archive did not contain a ${target.binary} binary`);

      await rm(finalPath, { force: true });
      try {
        await rename(binary, finalPath);
      } catch {
        await copyFile(binary, finalPath);
      }
      if (target.os !== 'windows') await chmod(finalPath, 0o755);
      await rm(archive, { force: true });
      await rm(checksumsPath, { force: true });
      await rm(extractDir, { recursive: true, force: true });
      await writeFile(
        join(dir, 'install.json'),
        JSON.stringify({ tag, asset: target.asset, installedAt: new Date().toISOString() }, null, 2),
      );

      const version = await this.readVersion(finalPath);
      if (!version) throw new Error('Downloaded Pegg binary failed its version check');
      progress?.(`Pegg ${version} installed.`, 100);
      this.log.info('Installed Pegg binary', finalPath, version);
      return this.use(finalPath);
    } catch (err) {
      await rm(archive, { force: true });
      throw err;
    }
  }

  private async resolveTag(_allowLatest: boolean): Promise<string> {
    const configured = getConfig().binaryVersion;
    if (configured && configured !== 'match-extension') {
      if (configured === 'latest') return 'latest';
      return configured.startsWith('v') ? configured : `v${configured}`;
    }
    const version = String(this.context.extension.packageJSON.version ?? '');
    if (/^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/.test(version)) {
      return `v${version}`;
    }
    return 'latest';
  }

  private cachedDir(tag: string): string {
    return join(this.context.globalStorageUri.fsPath, 'bin', tag.replace(/[^A-Za-z0-9._-]/g, '_'));
  }

  private cachedPath(tag: string): string {
    return join(this.cachedDir(tag), detectTarget().binary);
  }
}

async function locateBinary(root: string, name: string): Promise<string | undefined> {
  const stack = [root];
  while (stack.length > 0) {
    const current = stack.pop()!;
    let entries;
    try {
      entries = await readdir(current, { withFileTypes: true });
    } catch {
      continue;
    }
    for (const entry of entries) {
      const full = join(current, entry.name);
      if (entry.isDirectory()) stack.push(full);
      else if (entry.name === name) return resolve(full);
    }
  }
  return undefined;
}

export function peggHome(): string {
  return join(homedir(), '.pegg');
}
