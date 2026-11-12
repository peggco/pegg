import { ArrowRight, Check, Download } from 'lucide-react';
import { useT } from '../lib/i18n';
import { catalogTransport, initials } from '../lib/catalog';
import type { CatalogItem } from '../types';

export function CatalogCard({
  item,
  installed,
  busy,
  onOpen,
  onInstall,
}: {
  item: CatalogItem;
  installed: boolean;
  busy?: boolean;
  onOpen: () => void;
  onInstall: () => void;
}) {
  const t = useT();
  const transport = catalogTransport(item);
  const tags = item.tags?.slice(0, 2) ?? [];

  return (
    <div
      className="hub-card"
      role="button"
      tabIndex={0}
      onClick={onOpen}
      onKeyDown={(event) => {
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault();
          onOpen();
        }
      }}
    >
      <div className="hub-card-top">
        <span className="hub-avatar">{initials(item.name)}</span>
        <span className={`hub-status${item.official ? ' official' : ''}`}>
          <span className="hub-dot" />
          {item.official ? t('catalog.official') : t('catalog.community')}
        </span>
        <button
          className={`hub-install${installed ? ' done' : ''}`}
          disabled={busy || installed}
          onClick={(event) => {
            event.stopPropagation();
            onInstall();
          }}
        >
          {installed ? <Check size={12} /> : <Download size={12} />}
          {installed ? t('catalog.installed') : busy ? t('catalog.installing') : t('catalog.install')}
        </button>
      </div>
      <div className="hub-card-name">{item.name}</div>
      <div className="hub-card-desc">{item.description || item.source}</div>
      <div className="hub-tag-row">
        <span className="hub-kind-badge">{t(`catalog.kind.${item.kind}`)}</span>
        {item.version ? <span className="hub-license">v{item.version}</span> : null}
        {transport ? <span className="hub-license">{transport}</span> : null}
        {item.kind === 'plugins' && item.platforms?.length ? (
          <span className="hub-license">{item.platforms.join(' · ')}</span>
        ) : null}
        {tags.map((tag) => (
          <span className="hub-license" key={tag}>
            #{tag}
          </span>
        ))}
      </div>
      <div className="hub-card-foot">
        {item.license ? <span className="hub-license">{item.license}</span> : <span />}
        <ArrowRight size={14} className="hub-arrow" />
      </div>
    </div>
  );
}
