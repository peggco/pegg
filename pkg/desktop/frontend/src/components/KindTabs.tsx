import { useT } from '../lib/i18n';
import type { CatalogKind } from '../types';

const TABS: CatalogKind[] = ['skills', 'mcp', 'plugins'];

export function KindTabs({ tab, onChange }: { tab: CatalogKind; onChange: (tab: CatalogKind) => void }) {
  const t = useT();
  return (
    <div className="catalog-tabs" role="tablist" aria-label={t('catalog.title')}>
      {TABS.map((kind) => (
        <button
          key={kind}
          role="tab"
          aria-selected={tab === kind}
          className={`catalog-tab${tab === kind ? ' active' : ''}`}
          onClick={() => onChange(kind)}
        >
          {t(`catalog.kind.${kind}`)}
        </button>
      ))}
    </div>
  );
}
