import type { CatalogInstalled, CatalogItem } from '../types';

export function initials(name: string): string {
  const words = name
    .replace(/[^a-zA-Z0-9]+/g, ' ')
    .trim()
    .split(/\s+/)
    .filter(Boolean);
  if (words.length > 1) return (words[0][0] + words[1][0]).toUpperCase();
  return name.replace(/[^a-zA-Z0-9]/g, '').slice(0, 2).toUpperCase() || '?';
}

export function catalogKey(item: CatalogItem): string {
  return `${item.kind}:${item.id || item.skillId || item.name}`;
}

export function skillID(item: CatalogItem): string {
  return item.skillId || item.id;
}

export function catalogInstallCommand(item: CatalogItem): string {
  if (item.kind === 'skills') return `pegg skills add ${item.source}/${skillID(item)}`;
  if (item.kind === 'mcp') return `pegg mcp install ${item.id}`;
  return `pegg plugins install ${item.source || item.id}`;
}

export function catalogTransport(item: CatalogItem): string {
  if (item.transport) return item.transport;
  return item.transports?.join(' · ') ?? '';
}

function installedList(item: CatalogItem, installed: CatalogInstalled): string[] {
  if (item.kind === 'skills') return installed.skills;
  if (item.kind === 'mcp') return installed.mcp;
  return installed.plugins;
}

export function isCatalogInstalled(item: CatalogItem, installed: CatalogInstalled): boolean {
  const candidates = [item.id, item.name, item.skillId].filter((value): value is string => Boolean(value));
  const names = candidates.flatMap((value) => [value, value.split('/').pop() ?? value]).map((value) => value.toLowerCase());
  return installedList(item, installed).some((value) => names.includes(value.toLowerCase()));
}

export function catalogFilter(official: string, kindFilter: string, ...extra: string[]): string {
  return [official === 'all' ? '' : official, kindFilter, ...extra].filter(Boolean).join(',');
}
