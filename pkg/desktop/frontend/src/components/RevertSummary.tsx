import { useState, type MouseEvent } from "react";
import { Check, ChevronDown, ChevronUp, RotateCcw, X } from "lucide-react";
import { useT } from "../lib/i18n";
import { store, useStore } from "../state/store";
import { undoRevert } from "../state/actions";
import type { RevertFileChange } from "../state/chat";

function statusLabel(t: (key: string, vars?: Record<string, string | number>) => string, status: RevertFileChange["status"]): string {
  if (status === "conflict") return t("revert.status.conflict");
  if (status === "skipped") return t("revert.status.skipped");
  return t("revert.status.reverted");
}

export function RevertSummary() {
  const summary = useStore((state) => state.chat.revert);
  const t = useT();
  const [expanded, setExpanded] = useState(false);
  if (!summary) return null;

  const files = summary.files ?? [];
  const restored = summary.status === "restored";
  const restoring = summary.status === "undoing";
  const failed = summary.status === "error";
  const indent = files.length === 1 ? "revert.title.one" : "revert.title.other";
  const title = restored
    ? t("revert.restored")
    : files.length === 0
      ? t("revert.title.none")
      : t(indent, { count: files.length });

  const keepFocus = (event: MouseEvent) => event.preventDefault();

  return (
    <div className={`revert-summary ${summary.status}`} role="status" aria-live="polite">
      <div className="revert-summary-head">
        <span className="revert-summary-icon" aria-hidden="true">
          {restored ? <Check size={13} /> : <RotateCcw size={13} />}
        </span>
        <span className="revert-summary-title">{title}</span>
        {!restored && files.length > 0 ? (
          <span className="revert-summary-stat">
            <span className="add">+{summary.totalAdditions}</span>{" "}
            <span className="del">−{summary.totalDeletions}</span>
          </span>
        ) : null}
        <span className="revert-summary-spacer" />
        {!restored && files.length > 0 ? (
          <button
            type="button"
            className="revert-summary-iconbtn"
            aria-expanded={expanded}
            aria-label={t(expanded ? "revert.hideFiles" : "revert.showFiles")}
            title={t(expanded ? "revert.hideFiles" : "revert.showFiles")}
            onMouseDown={keepFocus}
            onClick={() => setExpanded((value) => !value)}
          >
            {expanded ? <ChevronUp size={13} /> : <ChevronDown size={13} />}
          </button>
        ) : null}
        {restored ? (
          <button
            type="button"
            className="revert-summary-iconbtn"
            aria-label={t("revert.dismiss")}
            title={t("revert.dismiss")}
            onMouseDown={keepFocus}
            onClick={() => store.setChat((chat) => ({ ...chat, revert: undefined }))}
          >
            <X size={13} />
          </button>
        ) : (
          <button
            type="button"
            className="revert-summary-undo"
            disabled={restoring || !summary.snapshotId}
            onMouseDown={keepFocus}
            onClick={() => void undoRevert()}
          >
            {restoring ? t("revert.undoing") : t("revert.undo")}
          </button>
        )}
      </div>

      {restored ? <div className="revert-summary-restored">{t("revert.restoredDetail")}</div> : null}
      {failed ? <div className="revert-summary-error">{summary.error || t("revert.undoFailed")}</div> : null}

      {!restored && expanded && files.length > 0 ? (
        <div className="revert-summary-files">
          {files.map((file) => {
            const status = file.status ?? "reverted";
            return (
              <div key={file.path} className={`revert-summary-file ${status}`}>
                <span className="revert-summary-path" title={file.path}>
                  {file.path}
                </span>
                <span className={`revert-summary-badge ${status}`}>{statusLabel(t, status)}</span>
                <span className="revert-summary-stat">
                  <span className="add">+{file.additions}</span>{" "}
                  <span className="del">−{file.deletions}</span>
                </span>
              </div>
            );
          })}
          {files.some((file) => file.status === "conflict") ? (
            <div className="revert-summary-note">
              {files.find((file) => file.status === "conflict")?.reason || t("revert.conflictHint")}
            </div>
          ) : null}
          {files.some((file) => file.status === "skipped") ? (
            <div className="revert-summary-note">
              {files.find((file) => file.status === "skipped")?.reason || t("revert.skippedHint")}
            </div>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
