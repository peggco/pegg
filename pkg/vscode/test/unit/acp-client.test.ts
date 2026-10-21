import { afterAll, beforeAll, describe, expect, it } from 'vitest';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { AcpClient } from '../../src/acp/client';
import { SessionListResult, SessionNewResult } from '../../src/acp/types';

const binary = process.env.PEGG_TEST_BINARY;
const log = {
  debug: () => undefined,
  info: () => undefined,
  warn: () => undefined,
};

describe.skipIf(!binary)('ACP client against the real binary', () => {
  let client: AcpClient;
  let cwd: string;
  let created: string | undefined;

  beforeAll(async () => {
    cwd = await mkdtemp(join(tmpdir(), 'pegg-vscode-test-'));
    client = new AcpClient({
      cwd,
      getBinary: async () => binary!,
      log,
      version: 'test',
    });
    await client.start();
  });

  afterAll(async () => {
    if (created) await client.request('session/delete', { sessionId: created }).catch(() => undefined);
    await client.stop();
    await rm(cwd, { recursive: true, force: true });
  });

  it('initializes with Pegg capabilities', async () => {
    const init = await client.initialize();
    expect(init.protocolVersion).toBe(1);
    expect(init.agentCapabilities.pegg.permissions).toBe(true);
  });

  it('lists models and sessions', async () => {
    const models = await client.request('pegg/models', {});
    expect(Array.isArray(models.models)).toBe(true);
    const sessions = await client.request<SessionListResult>('session/list', { cwd, all: true, limit: 5 });
    expect(Array.isArray(sessions.sessions)).toBe(true);
  });

  it('creates, renames, sets mode and deletes a session', async () => {
    const session = await client.request<SessionNewResult>('session/new', { cwd });
    expect(session.sessionId).toBeTruthy();
    expect(session.peggSessionId).toBeTruthy();
    created = session.peggSessionId;
    await client.request('session/rename', { sessionId: session.sessionId, title: 'ACP smoke' });
    await client.request('session/set_mode', { sessionId: session.sessionId, modeId: 'plan' });
    await client.request('session/delete', { sessionId: created });
    created = undefined;
  });
});
