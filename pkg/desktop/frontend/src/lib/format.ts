export function relativeTime(iso?: string): string {
  if (!iso) return '';
  const t = new Date(iso).getTime();
  if (Number.isNaN(t)) return '';
  const diff = Date.now() - t;
  const min = Math.round(diff / 60000);
  if (min < 1) return 'just now';
  if (min < 60) return `${min}m ago`;
  const hours = Math.round(min / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.round(hours / 24);
  if (days === 1) return 'yesterday';
  if (days < 7) return `${days}d ago`;
  return new Date(t).toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
}

export function dayGroup(iso?: string): string {
  if (!iso) return 'Earlier';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return 'Earlier';
  const now = new Date();
  const start = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();
  const t = d.getTime();
  if (t >= start) return 'Today';
  if (t >= start - 86400000) return 'Yesterday';
  if (t >= start - 7 * 86400000) return 'Previous 7 days';
  if (d.getFullYear() === now.getFullYear()) return d.toLocaleDateString(undefined, { month: 'long' });
  return d.toLocaleDateString(undefined, { month: 'short', year: 'numeric' });
}

export function compactNumber(n: number): string {
  if (!Number.isFinite(n)) return '0';
  if (n < 1000) return String(n);
  if (n < 1_000_000) return `${(n / 1000).toFixed(n < 10_000 ? 1 : 0)}k`;
  return `${(n / 1_000_000).toFixed(1)}m`;
}

export function formatCost(amount?: number): string {
  if (!amount || amount <= 0) return '';
  if (amount < 0.01) return `$${amount.toFixed(4)}`;
  return `$${amount.toFixed(2)}`;
}

export function shortPath(path: string, max = 46): string {
  if (!path) return '';
  const clean = path.replace(/\\/g, '/');
  if (clean.length <= max) return clean;
  const parts = clean.split('/');
  if (parts.length <= 2) return clean.slice(-max);
  return `${parts[0]}/…/${parts.slice(-2).join('/')}`;
}

export function baseName(path: string): string {
  const parts = (path ?? '').replace(/\\/g, '/').split('/');
  return parts[parts.length - 1] || path;
}

export function greeting(name?: string): string {
  const hour = new Date().getHours();
  const part = hour < 5 ? 'Good night' : hour < 12 ? 'Good morning' : hour < 18 ? 'Good afternoon' : 'Good evening';
  return name ? `${part}, ${name}` : part;
}

export function initials(value: string): string {
  const clean = (value || '').trim();
  if (!clean) return 'P';
  const parts = clean.split(/\s+/).slice(0, 2);
  return parts.map((p) => p[0]?.toUpperCase() ?? '').join('') || 'P';
}

export function humanSize(bytes: number): string {
  if (!bytes) return '0 B';
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

export function toolKindLabel(kind?: string): string {
  switch (kind) {
    case 'execute':
      return 'Terminal';
    case 'read':
      return 'Read';
    case 'edit':
      return 'Edit';
    case 'search':
      return 'Search';
    case 'fetch':
      return 'Web';
    case 'think':
      return 'Reasoning';
    default:
      return 'Tool';
  }
}
