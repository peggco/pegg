import { useT } from '../lib/i18n';

export function ClosingOverlay({ active }: { active: boolean }) {
  const t = useT();
  if (!active) return null;
  return (
    <div className="closing-overlay" role="status" aria-live="assertive">
      <div className="closing-card">
        <span className="spinner" aria-hidden="true" />
        <span>{t('closing.title')}</span>
      </div>
    </div>
  );
}
