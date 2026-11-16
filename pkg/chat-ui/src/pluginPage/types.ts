export interface PluginPageField {
  id: string;
  label?: string;
  placeholder?: string;
  value?: string;
  secret?: boolean;
  required?: boolean;
  options?: string[];
}

export interface PluginPageAction {
  id: string;
  label: string;
  description?: string;
  confirm?: boolean;
  fields?: PluginPageField[];
}

export interface PluginPageMetric {
  label: string;
  value: string;
  detail?: string;
  badge?: string;
}

export interface PluginPageItem {
  id?: string;
  title: string;
  detail?: string;
  badge?: string;
  disabled?: boolean;
  fields?: PluginPageField[];
  action?: string;
  column?: string;
  meta?: Record<string, string>;
}

export interface PluginPageSection {
  title: string;
  kind?: string;
  items: PluginPageItem[];
  note?: string;
}

export interface PluginPage {
  id: string;
  title: string;
  description?: string;
  owner: string;
  layout?: string;
  metrics?: PluginPageMetric[];
  sections: PluginPageSection[];
  actions: PluginPageAction[];
  settings?: PluginPageField[];
  saveAction?: string;
  footer?: string;
}

export interface PluginPageResult {
  output?: string;
  page?: PluginPage;
}

export interface PluginPagesResult {
  pages: PluginPage[];
}

export interface PluginPageGetResult {
  page: PluginPage;
}

export type PluginPageIncoming =
  | { type: 'init'; pages: PluginPage[]; page?: PluginPage }
  | { type: 'pages'; requestId?: string; pages: PluginPage[] }
  | { type: 'page'; requestId?: string; page: PluginPage }
  | { type: 'actionResult'; requestId: string; output?: string; page?: PluginPage; error?: string }
  | { type: 'error'; requestId?: string; error: string };

export type PluginPageOutgoing =
  | { type: 'ready' }
  | { type: 'list'; requestId: string }
  | { type: 'get'; requestId: string; owner: string; pageId: string }
  | { type: 'refresh'; requestId: string; owner: string; pageId: string }
  | { type: 'action'; requestId: string; owner: string; pageId: string; actionId: string; values: Record<string, string> };

export function pageKey(page: PluginPage): string {
  return `${page.owner}:${page.id}`;
}
