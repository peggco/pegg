import { api } from '../lib/bridge';
import type {
  AcpStatus,
  Attachment,
  BinaryProgress,
  ModelEntry,
  PromptAttachment,
  SessionListItem,
  SessionUpdate,
  Theme,
} from '../types';
import { resolveTheme, applyTheme } from '../lib/theme';
import {
  addAsk,
  addNotice,
  addPermission,
  appendUser,
  applyUpdates,
  emptyChat,
  pruneChat,
  resolveAsk,
  resolvePermission,
  setAskAnswer,
  settlePendingTools,
  type ChatItem,
  type ChatState,
} from './chat';
import { store, type UIState } from './store';

export function errorText(err: unknown): string {
  if (err instanceof Error) return err.message;
  if (typeof err === 'string') return err;
  try {
    return JSON.stringify(err);
  } catch {
    return 'Unknown error';
  }
}

export function applyResolvedTheme(themes: Theme[], prefs: UIState['prefs']): Theme | undefined {
  const theme = resolveTheme(themes, prefs.theme, prefs.themeMode);
  if (!theme) return undefined;
  applyTheme(theme, prefs.fontScale || 1);
  store.set({ theme });
  void api.ApplyWindowTheme(theme.dark).catch(() => undefined);
  return theme;
}

export async function bootstrap(): Promise<void> {
  try {
    const boot = await api.Bootstrap();
    const themes = boot.themes?.length ? boot.themes : [];
    const prefs = boot.prefs;
    store.set({
      ready: true,
      boot,
      themes,
      prefs,
      workspace: boot.workspace || boot.defaultWorkspace,
      recents: prefs.recents ?? [],
      binary: boot.binary,
      sidebarCollapsed: prefs.sidebarCollapsed,
      language: boot.language || 'en',
      status: { state: boot.binary.found ? 'connecting' : 'disconnected' },
    });
    applyResolvedTheme(themes, prefs);
    if (boot.binary.found) {
      await loadSessions();
      await loadModels();
      await loadCommandsCatalog();
    }
  } catch (err) {
    store.set({ ready: true, error: errorText(err) });
    store.toast('error', errorText(err));
  }
}

export async function loadSessions(): Promise<void> {
  const { workspace } = store.get();
  store.set({ sessionsLoading: true });
  void api.Log(`loadSessions workspace=${workspace}`).catch(() => undefined);
  try {
    const res = await api.LoadSessionsPage(workspace, true, 0, 30);
    store.set({ sessions: res.sessions ?? [], sessionsLoading: false, sessionsNextCursor: res.nextCursor });
  } catch (err) {
    store.set({ sessionsLoading: false });
    const text = errorText(err);
    void api.Log(`loadSessions failed: ${text}`).catch(() => undefined);
    if (!/not running|binary/i.test(text)) store.toast('warn', `Could not load sessions: ${text}`);
  }
}

export async function loadMoreSessions(): Promise<void> {
  const state = store.get();
  if (state.sessionsLoading || !state.sessionsNextCursor) return;
  store.set({ sessionsLoading: true });
  try {
    const res = await api.LoadSessionsPage(state.workspace, true, Number(state.sessionsNextCursor) || state.sessions.length, 30);
    store.set((current) => {
      const seen = new Set(current.sessions.map((item) => item.id));
      const extra = (res.sessions ?? []).filter((item) => !seen.has(item.id));
      return {
        sessions: [...current.sessions, ...extra],
        sessionsLoading: false,
        sessionsNextCursor: res.nextCursor,
      };
    });
  } catch (err) {
    store.set({ sessionsLoading: false });
    const text = errorText(err);
    if (!/not running|binary/i.test(text)) store.toast('warn', `Could not load sessions: ${text}`);
  }
}

export async function loadCommandsCatalog(): Promise<void> {
  try {
    const commands = await api.SlashCommands();
    store.set({ commandsCatalog: commands ?? [] });
  } catch {
    /* the session's server-provided commands remain available */
  }
}

export async function refreshRecents(): Promise<void> {
  try {
    store.set({ recents: await api.Recents() });
  } catch {
    /* ignore */
  }
}

export async function chooseWorkspace(): Promise<void> {
  try {
    const path = await api.PickWorkspace();
    if (!path) return;
    await setWorkspace(path);
  } catch (err) {
    store.toast('error', errorText(err));
  }
}

export async function setWorkspace(path: string): Promise<void> {
  try {
    const abs = await api.OpenWorkspace(path);
    store.set({ workspace: abs, view: 'welcome', subagentView: null, chat: emptyChat() });
    await refreshRecents();
    await loadSessions();
    await loadCommandsCatalog();
    const prefs = await api.SavePrefs({ ...store.get().prefs, workspace: abs });
    store.set({ prefs });
    store.toast('success', `Workspace: ${abs}`);
  } catch (err) {
    store.toast('error', errorText(err));
  }
}

export async function setTheme(name: string, mode: 'auto' | 'light' | 'dark'): Promise<void> {
  const state = store.get();
  const prefs = { ...state.prefs, theme: name, themeMode: mode };
  store.set({ prefs });
  applyResolvedTheme(state.themes, prefs);
  try {
    const saved = await api.SetTheme(name, mode);
    store.set({ prefs: saved });
  } catch (err) {
    store.toast('warn', errorText(err));
  }
}

export async function savePrefs(patch: Partial<UIState['prefs']>): Promise<void> {
  const prefs = { ...store.get().prefs, ...patch };
  store.set({ prefs });
  try {
    const saved = await api.SavePrefs(prefs);
    store.set({ prefs: saved });
  } catch (err) {
    store.toast('warn', errorText(err));
  }
}

export async function installBinary(): Promise<void> {
  store.set({ installing: true, installProgress: { stage: 'Starting', percent: 0 } });
  try {
    const st = await api.InstallBinary();
    store.set({ binary: st, installing: false, installProgress: undefined });
    if (st.found) {
      store.toast('success', `Pegg ${st.version ?? ''} installed`);
      await loadSessions();
    } else {
      store.toast('error', st.error ?? 'Install failed');
    }
  } catch (err) {
    store.set({ installing: false, installProgress: undefined });
    store.toast('error', errorText(err));
  }
}

export async function findBinary(): Promise<void> {
  try {
    const st = await api.FindBinary();
    store.set({ binary: st });
    if (st.found) {
      store.toast('success', `Found Pegg ${st.version ?? ''}`);
      await loadSessions();
    } else {
      store.toast('warn', st.error ?? 'Pegg binary not found');
    }
  } catch (err) {
    store.toast('error', errorText(err));
  }
}

export async function chooseBinary(): Promise<void> {
  try {
    const path = await api.PickBinary();
    if (!path) return;
    const st = await api.SetBinaryPath(path);
    store.set({ binary: st });
    store.toast('success', `Using Pegg ${st.version ?? ''}`);
    await loadSessions();
  } catch (err) {
    store.toast('error', errorText(err));
  }
}

export function setBinaryProgress(p: BinaryProgress): void {
  store.set({ installProgress: p });
}

export function setServerStatus(status: AcpStatus): void {
  store.set({ status });
  if (status.state === 'exited' || status.state === 'error') {
    store.setChat((chat) =>
      chat.running
        ? addNotice(
            { ...chat, running: false },
            status.message ? `Connection lost: ${status.message}` : 'Pegg server exited',
            'error',
          )
        : chat,
    );
  }
}

export function startNewChat(): void {
  void api.ResetSession().catch(() => undefined);
  store.set({
    view: 'welcome',
    subagentView: null,
    chat: emptyChat(),
    composerFocus: store.get().composerFocus + 1,
  });
}

function sessionListToChat(item: SessionListItem): ChatState {
  return {
    ...emptyChat(),
    acpId: item.id,
    peggId: item.id,
    title: item.title || 'Session',
    cwd: item.projectDir,
    mode: (item.mode as ChatState['mode']) || 'auto',
    provider: item.provider,
    model: item.model,
    loading: true,
    historyOldestSeq: 0,
    historyHasMore: false,
    historyLoading: false,
    usage: {
      used: item.usage?.total_tokens ?? 0,
      size: 0,
      cost: item.usage?.cost,
    },
  };
}

export async function openSession(item: SessionListItem): Promise<void> {
  store.set({ view: 'chat', subagentView: null, loadingSession: item.id, chat: sessionListToChat(item) });
  try {
    const info = await api.LoadSession({
      peggId: item.id,
      cwd: item.projectDir || store.get().workspace,
      title: item.title,
      mode: item.mode,
      provider: item.provider,
      model: item.model,
    });
    store.setChat((chat) =>
      chat.acpId === info.acpId
        ? {
            ...chat,
            acpId: info.acpId,
            peggId: info.peggId,
            cwd: info.cwd,
            title: info.title || chat.title,
            mode: (info.mode as ChatState['mode']) || chat.mode,
            provider: info.provider || chat.provider,
            model: info.model || chat.model,
            loading: false,
            historyOldestSeq: info.historyOldestSeq ?? chat.historyOldestSeq,
            historyHasMore: info.historyHasMore ?? chat.historyHasMore,
          }
        : chat,
    );
  } catch (err) {
    store.setChat((chat) => addNotice({ ...chat, loading: false }, errorText(err), 'error'));
    store.toast('error', errorText(err));
  } finally {
    store.set({ loadingSession: undefined });
  }
}

export async function renameSession(item: SessionListItem, title: string): Promise<void> {
  const clean = title.trim();
  if (!clean) return;
  try {
    await api.RenameSession(item.id, clean);
    store.set((s) => ({
      sessions: s.sessions.map((x) => (x.id === item.id ? { ...x, title: clean } : x)),
    }));
    store.setChat((chat) => (chat.peggId === item.id ? { ...chat, title: clean } : chat));
  } catch (err) {
    store.toast('error', errorText(err));
  }
}

export async function deleteSession(item: SessionListItem): Promise<void> {
  try {
    await api.DeleteSession(item.id);
    store.set((s) => ({ sessions: s.sessions.filter((x) => x.id !== item.id) }));
    if (store.get().chat.peggId === item.id) startNewChat();
  } catch (err) {
    store.toast('error', errorText(err));
  }
}

export interface MessageRef {
  messageId?: string;
  userMessageIndex?: number;
  text?: string;
}

function hasMessageRef(ref: MessageRef): boolean {
  return Boolean(ref.messageId) || ref.userMessageIndex !== undefined;
}

export async function forkMessage(ref: MessageRef): Promise<void> {
  const chat = store.get().chat;
  if (!chat.acpId || !hasMessageRef(ref)) return;
  if (chat.running) {
    store.toast('warn', 'Wait for Pegg to finish before forking.');
    return;
  }
  try {
    const res = await api.ForkSession(chat.acpId, ref.messageId ?? '', ref.userMessageIndex ?? -1);
    await loadSessions();
    await openSession({
      id: res.sessionId,
      title: res.title || 'Forked session',
      projectDir: chat.cwd,
      mode: chat.mode,
      provider: chat.provider,
      model: chat.model,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    });
    store.toast('success', 'Forked session');
  } catch (err) {
    store.toast('error', errorText(err));
  }
}

export async function revertMessage(ref: MessageRef): Promise<void> {
  const chat = store.get().chat;
  if (!chat.acpId || !hasMessageRef(ref)) return;
  if (chat.running) {
    store.toast('warn', 'Wait for Pegg to finish before reverting.');
    return;
  }
  try {
    const res = await api.RevertSession(chat.acpId, ref.messageId ?? '', ref.userMessageIndex ?? -1);
    const current = store.get().chat;
    store.setChat((c) => ({ ...c, items: c.items.slice(0, Math.max(0, c.items.length - res.removed)) }));
    if (current.acpId) {
      const item = store.get().sessions.find((s) => s.id === current.peggId);
      if (item) await openSession(item);
    }
    if (ref.text) {
      window.dispatchEvent(new CustomEvent('pegg:composer-text', { detail: ref.text }));
    }
    store.toast('success', `Reverted ${res.removed} message${res.removed === 1 ? '' : 's'}`);
  } catch (err) {
    store.toast('error', errorText(err));
  }
}

export interface SendInput {
  text: string;
  displayText?: string;
  attachments?: PromptAttachment[];
}

export async function sendPrompt(input: SendInput): Promise<void> {
  const state = store.get();
  const text = input.text;
  if (!text.trim() && !input.attachments?.length) return;
  if (!state.workspace) {
    store.toast('warn', 'Choose a project folder first');
    return;
  }

  store.set({ view: 'chat' });
  let chat = store.get().chat;
  if (chat.running) return;

  if (!chat.acpId) {
    store.setChat((c) => ({ ...c, loading: true }));
    try {
      const info = await api.NewSession({
        cwd: state.workspace,
        mode: chat.mode,
        provider: chat.provider,
        model: chat.model,
        reasoningEffort: chat.reasoningEffort,
      });
      chat = {
        ...store.get().chat,
        acpId: info.acpId,
        peggId: info.peggId,
        cwd: info.cwd,
        title: info.title || 'New session',
        mode: (info.mode as ChatState['mode']) || 'auto',
        provider: info.provider || store.get().chat.provider,
        model: info.model || store.get().chat.model,
        reasoningEffort: info.reasoningEffort || store.get().chat.reasoningEffort,
        permissionMode: info.permissionMode,
        commands: info.commands?.length ? info.commands : store.get().chat.commands,
        loading: false,
        running: true,
      };
    } catch (err) {
      store.setChat((c) => ({ ...c, loading: false, running: false }));
      store.setChat((c) => addNotice(c, errorText(err), 'error'));
      store.toast('error', errorText(err));
      return;
    }
  } else {
    chat = { ...chat, running: true };
  }

  const shownTitle =
    chat.title === 'New session' || !chat.title
      ? input.text.trim().slice(0, 64) || input.attachments?.[0]?.name || 'New session'
      : chat.title;
  chat = appendUser(chat, input.text, input.displayText ?? input.text);
  chat = { ...chat, title: shownTitle };
  store.setChat(() => chat);

  try {
    const result = await api.SendPrompt({
      sessionId: chat.acpId!,
      text,
      attachments: input.attachments,
    });
    const toolStatus = result?.stopReason === 'cancelled' ? 'cancelled' : 'completed';
    store.setChat((c) => ({ ...settlePendingTools(c, toolStatus), running: false }));
    if (result?.stopReason !== 'cancelled' && !document.hasFocus()) {
      const lastAssistant = [...store.get().chat.items]
        .reverse()
        .find((item): item is Extract<ChatItem, { kind: 'assistant' }> => item.kind === 'assistant' && item.text.trim().length > 0);
      const summary = lastAssistant?.text.replace(/\s+/g, ' ').trim().slice(0, 140) || 'Task completed';
      void api.Notify('Pegg finished', summary).catch(() => undefined);
    }
    if (result?.usage?.used) {
      store.setChat((c) => ({
        ...c,
        usage: {
          used: result.usage?.used ?? c.usage.used,
          size: result.usage?.size ?? c.usage.size,
          cost: result.usage?.cost?.amount ?? c.usage.cost,
        },
      }));
    }
    void loadSessions();
  } catch (err) {
    const message = errorText(err);
    store.setChat((c) => addNotice({ ...settlePendingTools(c, 'failed'), running: false }, message, 'error'));
    store.toast('error', message);
  }
}

export async function cancelPrompt(): Promise<void> {
  const chat = store.get().chat;
  if (!chat.acpId) return;
  try {
    await api.Cancel(chat.acpId);
  } catch (err) {
    store.toast('warn', errorText(err));
  }
}

export async function changeMode(mode: ChatState['mode']): Promise<void> {
  const chat = store.get().chat;
  store.setChat((c) => ({ ...c, mode }));
  if (!chat.acpId) return;
  try {
    const info = await api.SetMode(chat.acpId, mode);
    store.setChat((c) => ({ ...c, mode: (info.mode as ChatState['mode']) || mode }));
  } catch (err) {
    store.toast('error', errorText(err));
  }
}

export async function changeModel(provider: string, model: string): Promise<void> {
  store.setChat((c) => ({ ...c, provider, model }));
  const acpId = store.get().chat.acpId;
  if (!acpId) return;
  try {
    await api.SetModel(acpId, provider, model);
    store.toast('success', `Model: ${model}`);
  } catch (err) {
    store.toast('error', errorText(err));
  }
}

export async function changeReasoning(effort: string): Promise<void> {
  const chat = store.get().chat;
  store.setChat((c) => ({ ...c, reasoningEffort: effort }));
  if (!chat.acpId) return;
  try {
    const info = await api.SetReasoningEffort(chat.acpId, chat.provider ?? '', chat.model ?? '', effort);
    store.setChat((c) => ({ ...c, reasoningEffort: info.reasoningEffort || effort }));
  } catch (err) {
    store.toast('error', errorText(err));
  }
}

export async function loadModels(): Promise<void> {
  if (store.get().modelsLoading) return;
  store.set({ modelsLoading: true });
  try {
    const res = await api.ListModels('');
    const models: ModelEntry[] = res.models ?? [];
    store.set({ models, modelsLoading: false });
    if (!models.length) store.toast('warn', 'No models configured. Add a provider in Pegg settings.');
  } catch (err) {
    store.set({ modelsLoading: false });
    store.toast('warn', errorText(err));
  }
}

export async function respondPermission(requestId: string, optionId: string): Promise<void> {
  store.setChat((c) => resolvePermission(c, requestId));
  try {
    await api.RespondPermission(requestId, optionId);
  } catch (err) {
    store.toast('error', errorText(err));
  }
}

export async function respondAsk(requestId: string, answers: Record<string, string>): Promise<void> {
  store.setChat((c) => resolveAsk(c, requestId));
  try {
    await api.RespondAsk(requestId, answers);
  } catch (err) {
    store.toast('error', errorText(err));
  }
}

export function answerAsk(requestId: string, answers: Record<string, string>): void {
  store.setChat((c) => setAskAnswer(c, requestId, answers));
}

export async function attachFiles(): Promise<Attachment[]> {
  try {
    return await api.AttachFiles();
  } catch (err) {
    store.toast('error', errorText(err));
    return [];
  }
}

export async function readDropped(paths: string[]): Promise<Attachment[]> {
  const out: Attachment[] = [];
  for (const path of paths) {
    try {
      out.push(await api.ReadAttachment(path));
    } catch (err) {
      store.toast('warn', `${path}: ${errorText(err)}`);
    }
  }
  return out;
}

let recording = false;

export async function toggleRecording(): Promise<void> {
  try {
    if (!recording) {
      await api.RecordStart();
      recording = true;
      store.toast('info', 'Listening… press the mic again to stop');
    } else {
      recording = false;
      const res = await api.RecordStop();
      if (res?.text) {
        window.dispatchEvent(new CustomEvent('pegg:transcript', { detail: res.text }));
      }
    }
  } catch (err) {
    recording = false;
    store.toast('error', errorText(err));
  }
}

export function handleUpdates(sessionId: string, updates: SessionUpdate[]): void {
  const chat = store.get().chat;
  if (!chat.acpId) return;
  if (sessionId !== chat.acpId && sessionId !== chat.peggId) return;
  const history = updates.filter((u) => u.historyPage);
  const live = updates.filter((u) => !u.historyPage);
  store.setChat((c) => {
    let next = live.length ? applyUpdates(c, live) : c;
    if (history.length) {
      const temp = applyUpdates(
        { ...c, items: [], plan: [], permissions: [], asks: [], subagents: {} },
        history,
      );
      next = { ...next, items: [...temp.items, ...next.items], counter: Math.max(next.counter, temp.counter) };
    }
    if (!history.length && next.stickToBottom) next = pruneChat(next);
    if (updates.some((u) => u.sessionUpdate === 'agent_message_chunk' || u.sessionUpdate === 'user_message_chunk')) {
      next = settlePendingTools(next, 'completed');
    }
    if (updates.some((u) => u.sessionUpdate === 'error')) next = { ...next, running: false };
    return next;
  });
}

export async function loadOlderMessages(): Promise<void> {
  const chat = store.get().chat;
  if (!chat.acpId || !chat.historyHasMore || chat.historyLoading) return;
  store.setChat((c) => ({ ...c, historyLoading: true }));
  try {
    const res = await api.LoadHistory({
      sessionId: chat.acpId,
      cwd: chat.cwd,
      beforeSeq: chat.historyOldestSeq,
      limit: 50,
    });
    store.setChat((c) => ({
      ...c,
      historyOldestSeq: res.oldestSeq || c.historyOldestSeq,
      historyHasMore: res.hasMore,
      historyLoading: false,
    }));
  } catch (err) {
    store.setChat((c) => ({ ...c, historyLoading: false }));
    store.toast('error', errorText(err));
  }
}

export function setStickToBottom(stick: boolean): void {
  store.setChat((c) => (c.stickToBottom === stick ? c : { ...c, stickToBottom: stick }));
}

export function handlePermission(requestId: string, params: Parameters<typeof addPermission>[2]): void {
  store.setChat((c) => addPermission(c, requestId, params));
}

export function handleAsk(requestId: string, params: Parameters<typeof addAsk>[2]): void {
  store.setChat((c) => addAsk(c, requestId, params));
}

export function clearPending(): void {
  store.setChat((c) => ({ ...c, permissions: [], asks: [] }));
}

export function handleLoading(payload: { sessionId: string; loading: boolean }): void {
  const chat = store.get().chat;
  if (chat.peggId !== payload.sessionId && chat.acpId !== payload.sessionId) return;
  store.setChat((c) => ({ ...c, loading: payload.loading }));
  if (payload.loading) store.set({ loadingSession: payload.sessionId });
  else store.set({ loadingSession: undefined });
}

export function openSubagent(agentId: string): void {
  store.set({ subagentView: agentId });
}

export function closeSubagent(): void {
  store.set({ subagentView: null });
}

export function toggleSettings(open?: boolean): void {
  store.set((s) => ({ settingsOpen: open ?? !s.settingsOpen }));
}

export function togglePalette(open?: boolean): void {
  store.set((s) => ({ paletteOpen: open ?? !s.paletteOpen }));
}

export function toggleSidebar(): void {
  const collapsed = !store.get().sidebarCollapsed;
  store.set({ sidebarCollapsed: collapsed });
  void savePrefs({ sidebarCollapsed: collapsed });
}

export function focusComposer(): void {
  store.set((s) => ({ composerFocus: s.composerFocus + 1 }));
}

