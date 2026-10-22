import * as vscode from 'vscode';
import { execFile } from 'node:child_process';
import { constants } from 'node:fs';
import { access, chmod, copyFile, mkdir, readdir, readFile, rename, rm, stat, writeFile } from 'node:fs/promises';
import { homedir } from 'node:os';
import { join, resolve } from 'node:path';
import { promisify } from 'node:util';
import { getConfig, workspaceRoot } from '../config';
import { Logger } from '../logger';
import { parseChecksums, verifyChecksum } from './checksum';
import { downloadFileAny, extractArchive } from './download';
import { searchDirs } from './paths';
import { detectTarget } from './platform';

const execFileAsync = promisify(execFile);

export interface EnsureOptions {
  force?: boolean;
  progress?: (message: string, increment: number) => void;
}

export interface ProbeResult {
  path: string;
  ok: boolean;
  version?: string;
  error?: string;
}

export interface BinaryDiagnostic {
  platform: string;
  arch: string;
  extensionVersion: string;
  binaryVersionSetting: string;
  downloadBaseUrl: string;
  probeCwd: string;
  searchDirs: string[];
  candidates: ProbeResult[];
  resolved?: string;
  resolvedVersion?: string;
  error?: string;
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
      const probe = await this.verify(cfg.executablePath);
      if (!probe.ok) {
        throw new Error(
          `pegg.executablePath does not point to a working binary: ${cfg.executablePath}` +
            (probe.error ? ` (${probe.error})` : ''),
        );
      }
      return this.use(cfg.executablePath, probe.version);
    }

    if (!options.force) {
      const candidate = await this.findExisting();
      if (candidate) return this.use(candidate.path, candidate.version);
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

  async diagnose(): Promise<BinaryDiagnostic> {
    const cfg = getConfig();
    const target = detectTarget();
    const diagnostic: BinaryDiagnostic = {
      platform: `${process.platform}/${process.arch}`,
      arch: target.arch,
      extensionVersion: String(this.context.extension.packageJSON.version ?? 'unknown'),
      binaryVersionSetting: cfg.binaryVersion,
      downloadBaseUrl: this.downloadBase(),
      probeCwd: this.probeCwd(),
      searchDirs: await searchDirs(),
      candidates: [],
    };

    const candidates = await this.candidates();
    if (cfg.executablePath) {
      const probe = await this.verify(cfg.executablePath);
      diagnostic.candidates.unshift(probe);
      if (probe.ok) {
        diagnostic.resolved = cfg.executablePath;
        diagnostic.resolvedVersion = probe.version;
        return diagnostic;
      }
      diagnostic.error = `pegg.executablePath does not point to a working binary: ${cfg.executablePath} (${probe.error ?? 'unknown error'})`;
      return diagnostic;
    }

    for (const candidate of candidates) {
      const probe = await this.verify(candidate);
      diagnostic.candidates.push(probe);
      if (probe.ok) {
        diagnostic.resolved = candidate;
        diagnostic.resolvedVersion = probe.version;
        return diagnostic;
      }
    }

    diagnostic.error = 'No working pegg binary found.';
    return diagnostic;
  }

  private async use(path: string, version?: string): Promise<string> {
    this.cached = path;
    this.version = version ?? (await this.probe(path)).version;
    return path;
  }

  private async findExisting(): Promise<ProbeResult | undefined> {
    if (this.cached) {
      const probe = await this.verify(this.cached);
      if (probe.ok) return { ...probe, path: this.cached };
      this.cached = undefined;
    }

    for (const candidate of await this.candidates()) {
      const probe = await this.verify(candidate);
      if (probe.ok) return { ...probe, path: candidate };
    }
    return undefined;
  }

  private async candidates(): Promise<string[]> {
    const target = detectTarget();
    const root = workspaceRoot();
    const paths: string[] = [];
    if (root && getConfig().allowWorkspaceBinary) paths.push(join(root, 'bin', target.binary));
    for (const dir of await searchDirs()) paths.push(join(dir, target.binary));
    paths.push(this.cachedPath(await this.resolveTag()));
    return paths;
  }

  private async verify(path: string): Promise<ProbeResult> {
    if (!path) return { path, ok: false, error: 'empty path' };
    try {
      if (process.platform !== 'win32') await access(path, constants.X_OK);
      else await access(path, constants.F_OK);
      const info = await stat(path);
      if (!info.isFile()) return { path, ok: false, error: 'not a file' };
    } catch (err) {
      return { path, ok: false, error: describeError(err) };
    }
    const probe = await this.probe(path);
    if (!probe.version) {
      this.log.debug('binary probe failed', path, probe.error);
      return { path, ok: false, error: probe.error };
    }
    return { path, ok: true, version: probe.version };
  }

  private async probe(path: string): Promise<{ version?: string; error?: string }> {
    try {
      const { stdout, stderr } = await execFileAsync(path, ['version'], {
        timeout: 15000,
        cwd: this.probeCwd(),
      });
      const match = /pegg version\s+([^\s]+)/i.exec(stdout);
      if (match?.[1]) return { version: match[1] };
      const detail = firstLine(stderr) || firstLine(stdout) || 'no version output';
      return { error: `unexpected output: ${detail}` };
    } catch (err) {
      return { error: describeExecError(err) };
    }
  }

  private probeCwd(): string {
    const root = workspaceRoot();
    if (root) return root;
    return homedir();
  }

  private async download(progress?: (message: string, increment: number) => void): Promise<string> {
    const tag = await this.resolveTag();
    const target = detectTarget();
    const dir = this.cachedDir(tag);
    const finalPath = join(dir, target.binary);

    const existing = await this.verify(finalPath);
    if (existing.ok) return this.use(finalPath, existing.version);

    progress?.('Resolving Pegg release…', 0);
    const base = this.downloadBase();
    const archive = join(dir, target.asset);
    const checksumsPath = join(dir, 'checksums.txt');
    await mkdir(dir, { recursive: true });
    await rm(finalPath, { force: true });

    try {
      progress?.(`Downloading ${target.asset}…`, 5);
      await downloadFileAny(this.assetUrls(base, tag, target.asset), archive, (received, total) => {
        if (total > 0) {
          const pct = Math.min(70, 5 + Math.round((received / total) * 65));
          progress?.(`Downloading ${target.asset}… ${Math.round((received / total) * 100)}%`, pct);
        }
      });

      progress?.('Downloading checksums…', 72);
      await downloadFileAny(this.assetUrls(base, tag, 'checksums.txt'), checksumsPath);

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

      const probe = await this.probe(finalPath);
      if (!probe.version) {
        throw new Error(
          `Downloaded Pegg binary failed its version check: ${probe.error ?? 'unknown error'}`,
        );
      }
      progress?.(`Pegg ${probe.version} installed.`, 100);
      this.log.info('Installed Pegg binary', finalPath, probe.version);
      return this.use(finalPath, probe.version);
    } catch (err) {
      await rm(archive, { force: true });
      throw err;
    }
  }

  private downloadBase(): string {
    return (process.env.PEGG_DOWNLOAD_BASE_URL || getConfig().downloadBaseUrl).replace(/\/+$/, '');
  }

  private assetUrls(base: string, tag: string, asset: string): string[] {
    if (tag === 'latest') {
      const urls: string[] = [];
      if (base.endsWith('/download')) {
        urls.push(`${base.slice(0, -'/download'.length)}/latest/download/${asset}`);
      }
      urls.push(`${base}/latest/${asset}`);
      return urls;
    }
    return [`${base}/${tag}/${asset}`];
  }

  private async resolveTag(): Promise<string> {
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

function firstLine(value: string | undefined): string {
  if (!value) return '';
  return value
    .split('\n')
    .map((line) => line.trim())
    .find((line) => line.length > 0) ?? '';
}

function describeError(err: unknown): string {
  if (err && typeof err === 'object' && 'code' in err) {
    const withCode = err as { code?: string; message?: string };
    return `${withCode.code ?? 'error'}: ${withCode.message ?? 'unknown error'}`;
  }
  return err instanceof Error ? err.message : String(err);
}

function describeExecError(err: unknown): string {
  if (!err || typeof err !== 'object') return String(err);
  const exec = err as { code?: string | number; signal?: string; stderr?: string; stdout?: string; message?: string };
  const parts: string[] = [];
  if (typeof exec.code === 'number') parts.push(`exit code ${exec.code}`);
  if (exec.signal) parts.push(`signal ${exec.signal}`);
  const stderr = firstLine(exec.stderr);
  if (stderr) parts.push(stderr);
  const stdout = firstLine(exec.stdout);
  if (!stderr && stdout) parts.push(stdout);
  if (parts.length === 0) parts.push(exec.message ?? 'unknown error');
  return parts.join(' — ');
}

export function peggHome(): string {
  return join(homedir(), '.pegg');
}
