import type { ReactNode } from 'react';

export function PageHead({ title, description }: { title: string; description?: string }) {
  return (
    <div className="settings-page-head">
      <h2 className="settings-page-title">{title}</h2>
      {description ? <p className="settings-page-desc">{description}</p> : null}
    </div>
  );
}

export function Card({
  title,
  actions,
  children,
}: {
  title?: string;
  actions?: ReactNode;
  children: ReactNode;
}) {
  return (
    <section className="settings-card">
      {title || actions ? (
        <div className="settings-card-head">
          {title ? <span className="settings-card-title">{title}</span> : null}
          {actions ? <div className="settings-card-actions">{actions}</div> : null}
        </div>
      ) : null}
      {children}
    </section>
  );
}

export function Field({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: string;
  children: ReactNode;
}) {
  return (
    <div className="field">
      <label>{label}</label>
      {children}
      {hint ? <span className="desc">{hint}</span> : null}
    </div>
  );
}

export function Switch({
  checked,
  onChange,
  label,
}: {
  checked: boolean;
  onChange: (next: boolean) => void;
  label?: string;
}) {
  return (
    <button
      type="button"
      className={`switch${checked ? ' on' : ''}`}
      aria-label={label ?? 'Toggle'}
      aria-pressed={checked}
      onClick={() => onChange(!checked)}
    />
  );
}

export function Segmented<T extends string>({
  options,
  value,
  onChange,
}: {
  options: Array<{ value: T; label: string }>;
  value: T;
  onChange: (next: T) => void;
}) {
  return (
    <div className="segmented" role="group">
      {options.map((option) => (
        <button
          key={option.value}
          type="button"
          className={option.value === value ? 'active' : ''}
          onClick={() => onChange(option.value)}
        >
          {option.label}
        </button>
      ))}
    </div>
  );
}

export function Range({
  min,
  max,
  step,
  value,
  onChange,
  format,
}: {
  min: number;
  max: number;
  step: number;
  value: number;
  onChange: (next: number) => void;
  format?: (value: number) => string;
}) {
  return (
    <div className="range-row">
      <input
        type="range"
        min={min}
        max={max}
        step={step}
        value={value}
        onChange={(event) => onChange(Number(event.target.value))}
      />
      <span className="range-val">{format ? format(value) : value}</span>
    </div>
  );
}

export function Empty({ children }: { children: ReactNode }) {
  return <div className="empty-state">{children}</div>;
}

export function Pill({ children, accent = false }: { children: ReactNode; accent?: boolean }) {
  return <span className={`badge${accent ? ' accent' : ''}`}>{children}</span>;
}

export function SaveBar({
  onSave,
  saving,
  label = 'Save changes',
  children,
}: {
  onSave: () => void;
  saving?: boolean;
  label?: string;
  children?: ReactNode;
}) {
  return (
    <div className="settings-save-bar">
      {children}
      <span className="spacer" />
      <button className="btn primary" disabled={saving} onClick={onSave}>
        {saving ? 'Saving…' : label}
      </button>
    </div>
  );
}
