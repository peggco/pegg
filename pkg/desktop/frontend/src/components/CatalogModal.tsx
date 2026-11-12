import { useCallback, useEffect, useRef, useState } from 'react';
import { Search, X } from 'lucide-react';
import { useT } from '../lib/i18n';
import {
  catalogFilter,
  catalogKey,
  isCatalogInstalled,
  skillID,
} from '../lib/catalog';
import {
  errorText,
  fetchCatalogEntry,
  fetchCatalogInstalled,
  fetchCatalogReadme,
  installHubSkill,
  searchCatalog,
  setCatalogTab,
  toggleCatalog,
} from '../state/actions';
import { store, useStore } from '../state/store';
import type {
  CatalogEntry,
  CatalogInstalled,
  CatalogItem,
  CatalogKind,
  CatalogSearchResult,
} from '../types';
import { CatalogCard } from './CatalogCard';
import { CatalogDetail } from './CatalogDetail';
import { KindTabs } from './KindTabs';

type Filter = 'all' | 'official' | 'community';

const PAGE_SIZE = 100;
const EMPTY_INSTALLED: CatalogInstalled = { skills: [], mcp: [], plugins: [] };

export function CatalogModal() {
  const open = useStore((s) => s.catalogOpen);
  const tab = useStore((s) => s.catalogTab);
  const platform = useStore((s) => s.boot?.platform ?? 'linux');
  const t = useT();
  const [filter, setFilter] = useState<Filter>('all');
  const [query, setQuery] = useState('');
  const [items, setItems] = useState<CatalogItem[]>([]);
  const [nextCursor, setNextCursor] = useState<number | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [selected, setSelected] = useState<CatalogItem | null>(null);
  const [entry, setEntry] = useState<CatalogEntry | null>(null);
  const [readme, setReadme] = useState('');
  const [readmeLoading, setReadmeLoading] = useState(false);
  const [installed, setInstalled] = useState<CatalogInstalled>(EMPTY_INSTALLED);
  const [busy, setBusy] = useState<string | null>(null);
  const requestID = useRef(0);
  const searchRef = useRef<HTMLInputElement>(null);

  const load = useCallback(
    async (
      kind: CatalogKind,
      value: string,
      official: Filter,
      kindValue: string,
      cursor = 0,
      append = false,
    ) => {
      const id = ++requestID.current;
      setLoading(true);
      try {
        const res: CatalogSearchResult = await searchCatalog(
          kind,
          value.trim(),
          catalogFilter(official, kindValue),
          PAGE_SIZE,
          cursor,
        );
        if (id !== requestID.current) return;
        setItems((prev) => (append ? [...prev, ...res.items] : res.items));
        setNextCursor(res.nextCursor ?? null);
        setError('');
      } catch (err) {
        if (id === requestID.current) setError(errorText(err));
      } finally {
        if (id === requestID.current) setLoading(false);
      }
    },
    [],
  );

  const secondaryFilter = tab === 'plugins' ? platform : '';

  useEffect(() => {
    if (!open) return;
    const timer = window.setTimeout(
      () => void load(tab, query, filter, secondaryFilter),
      query.trim() ? 300 : 0,
    );
    return () => window.clearTimeout(timer);
  }, [open, tab, query, filter, secondaryFilter, load]);

  useEffect(() => {
    if (!open) {
      setSelected(null);
      setEntry(null);
      setReadme('');
      return;
    }
    let alive = true;
    fetchCatalogInstalled()
      .then((res) => {
        if (alive) setInstalled(res ?? EMPTY_INSTALLED);
      })
      .catch(() => {
        if (alive) setInstalled(EMPTY_INSTALLED);
      });
    return () => {
      alive = false;
    };
  }, [open]);

  useEffect(() => {
    if (!selected) {
      setEntry(null);
      setReadme('');
      return;
    }
    let alive = true;
    setEntry(null);
    setReadme('');
    setReadmeLoading(true);
    fetchCatalogEntry(selected.kind, selected.id)
      .then((res) => {
        if (alive) setEntry(res);
      })
      .catch(() => undefined);
    fetchCatalogReadme(selected.kind, selected.id)
      .then((text) => {
        if (alive) setReadme(text);
      })
      .catch(() => {
        if (alive) setReadme('');
      })
      .finally(() => {
        if (alive) setReadmeLoading(false);
      });
    return () => {
      alive = false;
    };
  }, [selected]);

  const close = useCallback(() => {
    toggleCatalog(false);
    setSelected(null);
    setEntry(null);
    setReadme('');
  }, []);

  useEffect(() => {
    if (!open) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault();
        close();
        return;
      }
      const target = event.target as HTMLElement | null;
      const typing =
        target?.tagName === 'INPUT' || target?.tagName === 'TEXTAREA' || target?.isContentEditable === true;
      if (event.key === '/' && !typing && !selected) {
        event.preventDefault();
        searchRef.current?.focus();
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [open, selected, close]);

  const changeTab = (kind: CatalogKind) => {
    if (kind === tab) return;
    setCatalogTab(kind);
    setSelected(null);
    setEntry(null);
    setReadme('');
    setQuery('');
    setFilter('all');
    setItems([]);
    setNextCursor(null);
  };

  const markInstalled = (item: CatalogItem) => {
    setInstalled((current) => {
      const values = item.kind === 'skills' ? current.skills : item.kind === 'mcp' ? current.mcp : current.plugins;
      const next = Array.from(new Set([...values, item.name || item.id]));
      if (item.kind === 'skills') return { ...current, skills: next };
      if (item.kind === 'mcp') return { ...current, mcp: next };
      return { ...current, plugins: next };
    });
  };

  const quickInstall = async (item: CatalogItem) => {
    if (item.kind !== 'skills') {
      setSelected(item);
      return;
    }
    const key = catalogKey(item);
    setBusy(key);
    try {
      await installHubSkill(item.source, skillID(item), 'global');
      markInstalled(item);
    } catch (err) {
      store.toast('error', errorText(err));
    } finally {
      setBusy(null);
    }
  };

  const activeFilter = filter;

  return (
    <div
      className={`overlay${open ? '' : ' hidden'}`}
      aria-hidden={!open}
      onMouseDown={(event) => event.target === event.currentTarget && open && close()}
    >
      <div className="modal catalog-modal" role="dialog" aria-modal="true" aria-label={t('catalog.title')}>
        <div className="modal-head">
          <span className="modal-title">{t('catalog.title')}</span>
          <span className="badge">Pegg Hub</span>
          <div className="spacer" />
          <KindTabs tab={tab} onChange={changeTab} />
          <button className="icon-btn" aria-label={t('common.close')} onClick={close}>
            <X size={16} />
          </button>
        </div>
        <div className="modal-body">
          {selected ? (
            <CatalogDetail
              item={selected}
              entry={entry}
              readme={readme}
              readmeLoading={readmeLoading}
              installed={isCatalogInstalled(selected, installed)}
              onBack={() => setSelected(null)}
              onInstalled={markInstalled}
            />
          ) : (
            <div className="hub-pane">
              <div className="hub-hero">
                <div className="hub-eyebrow">{t('catalog.directory')}</div>
                <h1 className="hub-title">{t(`catalog.kind.${tab}`)}</h1>
                <p className="hub-sub">{t('catalog.subtitle')}</p>
              </div>

              <div className="hub-toolbar">
                <div className="hub-search">
                  <Search size={15} />
                  <input
                    ref={searchRef}
                    value={query}
                    placeholder={t('catalog.search')}
                    aria-label={t('catalog.search')}
                    onChange={(event) => setQuery(event.target.value)}
                  />
                </div>
                <div className="hub-filter">
                  {(['all', 'official', 'community'] as Filter[]).map((value) => (
                    <button
                      key={value}
                      className={activeFilter === value ? 'active' : ''}
                      onClick={() => setFilter(value)}
                    >
                      {t(`catalog.${value}`)}
                    </button>
                  ))}
                </div>
              </div>

              <div className="hub-meta">
                <span>
                  {loading && !items.length
                    ? t('common.loading')
                    : `${items.length}${nextCursor ? '+' : ''} ${t('catalog.count')}`}
                </span>
              </div>

              {error ? <div className="notice error">{error}</div> : null}

              {!loading && !items.length && !error ? <div className="hub-empty">{t('catalog.noResults')}</div> : null}

              <div className="hub-grid">
                {loading && !items.length
                  ? Array.from({ length: 6 }, (_, index) => (
                      <div className="hub-card catalog-skeleton" key={index} aria-hidden="true">
                        <span className="sk-avatar" />
                        <span className="sk-line" />
                        <span className="sk-line short" />
                        <span className="sk-line" />
                      </div>
                    ))
                  : items.map((item) => (
                      <CatalogCard
                        key={catalogKey(item)}
                        item={item}
                        installed={isCatalogInstalled(item, installed)}
                        busy={busy === catalogKey(item)}
                        onOpen={() => setSelected(item)}
                        onInstall={() => void quickInstall(item)}
                      />
                    ))}
              </div>

              {nextCursor ? (
                <div className="hub-more">
                  <button
                    className="btn small"
                    disabled={loading}
                    onClick={() => void load(tab, query, filter, secondaryFilter, nextCursor, true)}
                  >
                    {loading ? t('common.loading') : t('catalog.loadMore')}
                  </button>
                </div>
              ) : null}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
