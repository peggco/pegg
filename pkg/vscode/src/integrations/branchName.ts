export interface BranchVars {
  number?: number | string;
  iid?: number | string;
  slug?: string;
  title?: string;
  id?: string;
  provider?: string;
}

const MAX_SLUG = 40;

export function slugify(title: string): string {
  const plain = title
    .replace(/\[[^\]]*\]/g, ' ')
    .replace(/\([^)]*\)/g, ' ')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
  if (plain.length <= MAX_SLUG) return plain;
  const cut = plain.slice(0, MAX_SLUG);
  const lastDash = cut.lastIndexOf('-');
  return (lastDash > 20 ? cut.slice(0, lastDash) : cut).replace(/-+$/g, '');
}

function numberValue(vars: BranchVars): string | undefined {
  const raw = vars.number ?? vars.iid;
  if (raw === undefined || raw === null) return undefined;
  const text = String(raw).trim();
  return text || undefined;
}

export function renderBranchTemplate(template: string, vars: BranchVars): string {
  const number = numberValue(vars);
  const slug = slugify(vars.slug ?? vars.title ?? '');
  const title = slug;
  const id = (vars.id ?? '').trim().toLowerCase();
  const provider = (vars.provider ?? '').trim().toLowerCase();

  const rendered = template
    .replace(/\{number\}/g, number ?? '')
    .replace(/\{iid\}/g, number ?? '')
    .replace(/\{slug\}/g, slug)
    .replace(/\{title\}/g, title)
    .replace(/\{id\}/g, id)
    .replace(/\{provider\}/g, provider)
    .replace(/[^A-Za-z0-9/_.-]+/g, '-')
    .replace(/-+/g, '-')
    .replace(/^[-/]+|[-/]+$/g, '')
    .replace(/\/{2,}/g, '/');

  if (!rendered || rendered === '-' || rendered === '.') {
    return slug || number || '';
  }
  return rendered;
}

export function branchNameFor(template: string, vars: BranchVars): string {
  const rendered = renderBranchTemplate(template, vars);
  if (rendered) return rendered;
  return slugify(vars.title ?? '') || 'work';
}
