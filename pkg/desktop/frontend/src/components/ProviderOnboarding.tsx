import { useCallback, useEffect, useRef, useState } from "react";
import { ExternalLink, KeyRound, LogIn, Settings2 } from "lucide-react";
import { api, onPeggStatus } from "../lib/bridge";
import { useT } from "../lib/i18n";
import {
  applyRecommendedDefaults,
  errorText,
  fetchRecommendedDefaults,
  openSettings,
  refreshProviderSetup,
  waitForModels,
} from "../state/actions";
import { store } from "../state/store";
import type { ForeverStatus, RecommendedDefaults } from "../types";

const PEGG_FOREVER_BASE_URL = "https://forever.pegg.dev/v1";

type Step = "choices" | "login" | "key" | "defaults" | "models";

function statusLabel(t: (key: string) => string, state: string): string {
  switch (state) {
    case "starting":
      return t("onboarding.starting");
    case "waiting":
      return t("onboarding.waiting");
    case "approving":
      return t("onboarding.approving");
    case "done":
      return t("onboarding.done");
    default:
      return state;
  }
}

export function ProviderOnboarding() {
  const t = useT();
  const [step, setStep] = useState<Step>("choices");
  const [status, setStatus] = useState<ForeverStatus | null>(null);
  const [apiKey, setApiKey] = useState("");
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const [defaults, setDefaults] = useState<RecommendedDefaults | null>(null);
  const [defaultsBusy, setDefaultsBusy] = useState(false);
  const finished = useRef(false);

  const finish = useCallback(async () => {
    setStep("models");
    const ready = await waitForModels(60000);
    if (!ready) {
      store.toast(
        "warn",
        "Models are still loading. You can retry from the model menu.",
      );
    }
    await refreshProviderSetup();
    store.set((current) => ({
      providerSetup: {
        configured: true,
        dismissed: current.providerSetup?.dismissed ?? false,
      },
    }));
    setStep("choices");
  }, []);

  const loadDefaults = useCallback(async () => {
    setStep("defaults");
    setDefaultsBusy(true);
    setError("");
    try {
      const result = await fetchRecommendedDefaults();
      if (result.error || !result.available) {
        await finish();
        return;
      }
      setDefaults({
        ...result,
        changes: result.changes ?? [],
        skipped: result.skipped ?? [],
      });
    } catch (err) {
      setError(errorText(err));
      await finish();
    } finally {
      setDefaultsBusy(false);
    }
  }, [finish]);

  const handleStatus = useCallback(
    (next: ForeverStatus) => {
      setStatus(next);
      if (next.state === "done" && !finished.current) {
        finished.current = true;
        void loadDefaults();
      }
    },
    [loadDefaults],
  );

  useEffect(() => {
    if (step !== "login") return;
    return onPeggStatus(handleStatus);
  }, [step, handleStatus]);

  const startLogin = async () => {
    setError("");
    finished.current = false;
    setStatus({
      state: "starting",
      logs: [],
      configured: false,
      provider: "pegg-forever",
    });
    setStep("login");
    try {
      const next = await api.PeggLogin();
      handleStatus(next);
    } catch (err) {
      setError(errorText(err));
      setStatus((current) => ({
        ...(current ?? {
          logs: [],
          configured: false,
          provider: "pegg-forever",
        }),
        state: "error",
        error: errorText(err),
      }));
    }
  };

  const cancelLogin = async () => {
    try {
      await api.PeggCancel();
    } catch (err) {
      setError(errorText(err));
    }
    setStatus(null);
    setError("");
    setStep("choices");
  };

  const saveKey = async () => {
    const key = apiKey.trim();
    if (!key) {
      setError(t("onboarding.apiKeyRequired"));
      return;
    }
    setSaving(true);
    setError("");
    try {
      await api.AddProvider("pegg-forever", key, PEGG_FOREVER_BASE_URL);
      await loadDefaults();
    } catch (err) {
      setError(errorText(err));
    } finally {
      setSaving(false);
    }
  };

  const configureMyself = async () => {
    setError("");
    try {
      await api.DismissProviderSetup();
      store.set((current) => ({
        providerSetup: {
          configured: current.providerSetup?.configured ?? false,
          dismissed: true,
        },
      }));
      openSettings("general");
    } catch (err) {
      setError(errorText(err));
    }
  };

  const saveDefaults = async () => {
    try {
      setError("");
      await applyRecommendedDefaults();
      await finish();
    } catch (err) {
      setError(errorText(err));
    }
  };

  if (step === "models") {
    return (
      <div className="onboarding">
        <h1>{t("onboarding.modelsTitle")}</h1>
        <p>{t("onboarding.modelsWaiting")}</p>
      </div>
    );
  }

  if (step === "defaults") {
    return (
      <div className="onboarding">
        <h1>{t("onboarding.defaultsTitle")}</h1>
        <p>{t("onboarding.defaultsSubtitle")}</p>
        {defaultsBusy ? (
          <p>{t("onboarding.defaultsLoading")}</p>
        ) : (
          <>
            <div className="steps">
              {(defaults?.changes ?? []).map((change, index) => (
                <div className="step" key={`change-${index}`}>
                  {change}
                </div>
              ))}
              {(defaults?.skipped ?? []).length ? (
                <div className="step" style={{ color: "var(--pg-muted)" }}>
                  {t("onboarding.defaultsSkipped")}
                </div>
              ) : null}
              {(defaults?.skipped ?? []).map((skipped, index) => (
                <div
                  className="step"
                  key={`skip-${index}`}
                  style={{ color: "var(--pg-muted)" }}
                >
                  {skipped}
                </div>
              ))}
            </div>
            <div className="actions">
              <button
                className="btn primary"
                onClick={() => void saveDefaults()}
              >
                {t("onboarding.defaultsApply")}
              </button>
              <button className="btn ghost" onClick={() => void finish()}>
                {t("onboarding.defaultsSkip")}
              </button>
            </div>
          </>
        )}
        {error ? (
          <div className="notice error">{t("onboarding.error", { error })}</div>
        ) : null}
      </div>
    );
  }

  if (step === "login") {
    const state = status?.state ?? "starting";
    return (
      <div className="onboarding">
        <h1>{t("onboarding.loginTitle")}</h1>
        <p>{t("onboarding.waiting")}</p>
        <div className="steps">
          <div className="step">
            <span className="num">●</span>
            {statusLabel(t, state)}
          </div>
          {status?.url ? (
            <div
              className="step"
              style={{ flexDirection: "column", alignItems: "flex-start" }}
            >
              <code style={{ overflowWrap: "anywhere" }}>{status.url}</code>
            </div>
          ) : null}
        </div>
        {status?.error || error ? (
          <div className="notice error">
            {t("onboarding.error", { error: status?.error || error })}
          </div>
        ) : null}
        <div className="actions">
          {status?.url ? (
            <button
              className="btn"
              onClick={() => void api.OpenExternal(status.url ?? "")}
            >
              <ExternalLink size={14} />
              {t("onboarding.openBrowser")}
            </button>
          ) : null}
          <button className="btn ghost" onClick={() => void cancelLogin()}>
            {t("onboarding.cancel")}
          </button>
        </div>
      </div>
    );
  }

  if (step === "key") {
    return (
      <div className="onboarding">
        <h1>{t("onboarding.apiKeyTitle")}</h1>
        <p>{t("onboarding.apiKeyDetail")}</p>
        <div className="steps">
          <div
            className="step"
            style={{ flexDirection: "column", alignItems: "stretch", gap: 8 }}
          >
            <label
              htmlFor="provider-onboarding-key"
              style={{ fontSize: 12, color: "var(--pg-muted)" }}
            >
              {t("onboarding.apiKeyLabel")}
            </label>
            <input
              id="provider-onboarding-key"
              className="input"
              type="password"
              autoComplete="off"
              value={apiKey}
              placeholder={t("onboarding.apiKeyPlaceholder")}
              onChange={(event) => setApiKey(event.target.value)}
            />
            <span style={{ fontSize: 11.5, color: "var(--pg-muted)" }}>
              {t("onboarding.apiKeyHint")}
            </span>
          </div>
        </div>
        {error ? (
          <div className="notice error">{t("onboarding.error", { error })}</div>
        ) : null}
        <div className="actions">
          <button
            className="btn primary"
            disabled={saving}
            onClick={() => void saveKey()}
          >
            <KeyRound size={14} />
            {saving ? t("onboarding.saving") : t("onboarding.save")}
          </button>
          <button
            className="btn ghost"
            disabled={saving}
            onClick={() => {
              setError("");
              setStep("choices");
            }}
          >
            {t("onboarding.back")}
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="onboarding">
      <h1>{t("onboarding.title")}</h1>
      <p>{t("onboarding.subtitle")}</p>
      <div className="steps">
        <button
          className="step"
          style={{ textAlign: "left", cursor: "pointer" }}
          onClick={() => void startLogin()}
        >
          <LogIn size={16} />
          <span>
            <strong>{t("onboarding.login")}</strong>
            <span
              style={{
                display: "block",
                color: "var(--pg-muted)",
                fontSize: 12,
              }}
            >
              {t("onboarding.loginDetail")}
            </span>
          </span>
        </button>
        <button
          className="step"
          style={{ textAlign: "left", cursor: "pointer" }}
          onClick={() => setStep("key")}
        >
          <KeyRound size={16} />
          <span>
            <strong>{t("onboarding.apiKey")}</strong>
            <span
              style={{
                display: "block",
                color: "var(--pg-muted)",
                fontSize: 12,
              }}
            >
              {t("onboarding.apiKeyDetail")}
            </span>
          </span>
        </button>
        <button
          className="step"
          style={{ textAlign: "left", cursor: "pointer" }}
          onClick={() => void configureMyself()}
        >
          <Settings2 size={16} />
          <span>
            <strong>{t("onboarding.configure")}</strong>
            <span
              style={{
                display: "block",
                color: "var(--pg-muted)",
                fontSize: 12,
              }}
            >
              {t("onboarding.configureDetail")}
            </span>
          </span>
        </button>
      </div>
      {error ? (
        <div className="notice error">{t("onboarding.error", { error })}</div>
      ) : null}
    </div>
  );
}
