export type GitProvider = 'github' | 'bitbucket' | 'gitlab' | 'unknown';

export interface RemoteInfo {
  host: string;
  owner: string;
  repo: string;
  protocol: 'https' | 'ssh' | 'file' | 'unknown';
  provider: GitProvider;
}

const KNOWN_HOSTS: Record<string, GitProvider> = {
  'github.com': 'github',
  'www.github.com': 'github',
  'bitbucket.org': 'bitbucket',
  'www.bitbucket.org': 'bitbucket',
  'gitlab.com': 'gitlab',
  'www.gitlab.com': 'gitlab',
};

export function providerForHost(host: string): GitProvider {
  return KNOWN_HOSTS[host.toLowerCase()] ?? 'unknown';
}

export function detectProvider(host: string, override?: string): GitProvider {
  const wanted = (override ?? '').trim().toLowerCase();
  if (wanted === 'github' || wanted === 'bitbucket' || wanted === 'gitlab') return wanted;
  return providerForHost(host);
}

function splitPath(path: string): { owner: string; repo: string } | undefined {
  const cleaned = path
    .replace(/^\/+/, '')
    .replace(/\.git$/i, '')
    .replace(/\/+$/, '');
  const parts = cleaned.split('/').filter(Boolean);
  if (parts.length < 2) return undefined;
  return { owner: parts.slice(0, -1).join('/'), repo: parts[parts.length - 1] };
}

export function parseRemoteUrl(url: string, override?: string): RemoteInfo | undefined {
  const value = url.trim();
  if (!value) return undefined;

  const scp = /^(?:[^@/]+@)?([^:/]+):(?!\/\/)(.+)$/.exec(value);
  if (scp && !value.includes('://')) {
    const path = splitPath(scp[2]);
    if (!path) return undefined;
    return {
      host: scp[1].toLowerCase(),
      ...path,
      protocol: 'ssh',
      provider: detectProvider(scp[1], override),
    };
  }

  let parsed: URL;
  try {
    parsed = new URL(value);
  } catch {
    if (value.startsWith('/') || value.startsWith('file://')) {
      const path = splitPath(value.replace(/^file:\/\//, ''));
      if (!path) return undefined;
      return { host: 'localhost', ...path, protocol: 'file', provider: detectProvider('localhost', override) };
    }
    return undefined;
  }

  const path = splitPath(parsed.pathname);
  if (!path) return undefined;
  const protocol = parsed.protocol === 'https:' || parsed.protocol === 'http:' ? 'https' :
    parsed.protocol === 'ssh:' || parsed.protocol === 'git+ssh:' ? 'ssh' : 'unknown';
  return {
    host: parsed.hostname.toLowerCase(),
    ...path,
    protocol,
    provider: detectProvider(parsed.hostname, override),
  };
}

export function providerLabel(provider: GitProvider): string {
  switch (provider) {
    case 'github':
      return 'GitHub';
    case 'bitbucket':
      return 'Bitbucket';
    case 'gitlab':
      return 'GitLab';
    default:
      return 'Unknown';
  }
}
