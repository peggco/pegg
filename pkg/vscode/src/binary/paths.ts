import { readFile } from 'node:fs/promises';
import { homedir } from 'node:os';
import { delimiter, join } from 'node:path';

const PROFILE_FILES = ['.zshenv', '.zprofile', '.zshrc', '.bash_profile', '.bashrc', '.profile'];

const PATH_ASSIGNMENT = /(?:^|[\s;])(?:export\s+)?(?:PATH|path)\s*=\s*("([^"]*)"|'([^']*)'|([^\s;#]+))/g;

export function extraSearchPaths(platform = process.platform, home = homedir()): string[] {
  const dirs = [join(home, '.local', 'bin'), join(home, 'bin'), join(home, '.pegg', 'bin')];
  if (platform === 'darwin') {
    dirs.unshift('/opt/homebrew/bin', '/usr/local/bin', '/opt/local/bin');
  } else if (platform === 'linux') {
    dirs.unshift('/usr/local/bin', '/snap/bin');
  } else if (platform === 'win32') {
    return [];
  }
  return dirs;
}

export async function profileSearchPaths(home = homedir()): Promise<string[]> {
  const dirs: string[] = [];
  for (const file of PROFILE_FILES) {
    const content = await readFile(join(home, file), 'utf8').catch(() => undefined);
    if (!content) continue;
    for (const value of pathValues(content)) {
      for (const dir of value.split(':')) {
        const expanded = expandHome(dir.trim(), home);
        if (expanded) dirs.push(expanded);
      }
    }
  }
  return dirs;
}

export async function searchDirs(platform = process.platform, home = homedir()): Promise<string[]> {
  const dirs: string[] = [];
  const push = (dir: string | undefined) => {
    if (dir && !dirs.includes(dir)) dirs.push(dir);
  };
  for (const dir of (process.env.PATH ?? '').split(delimiter)) push(dir);
  for (const dir of extraSearchPaths(platform, home)) push(dir);
  for (const dir of await profileSearchPaths(home)) push(dir);
  return dirs;
}

function pathValues(content: string): string[] {
  const values: string[] = [];
  for (const match of content.matchAll(PATH_ASSIGNMENT)) {
    const value = match[2] ?? match[3] ?? match[4] ?? '';
    if (value) values.push(value);
  }
  return values;
}

function expandHome(dir: string, home: string): string | undefined {
  if (!dir) return undefined;
  let out = dir;
  if (out.startsWith('~')) out = join(home, out.slice(1).replace(/^\//, ''));
  out = out.replace(/\$\{?HOME\}?/g, home);
  if (out.includes('$')) return undefined;
  return out;
}
