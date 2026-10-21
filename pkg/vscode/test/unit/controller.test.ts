import { afterAll, beforeAll, describe, expect, it } from 'vitest';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { AcpClient } from '../../src/acp/client';
import { BinaryManager } from '../../src/binary/manager';
import { ChatController } from '../../src/chat/controller';
import { Logger } from '../../src/logger';
import { Uri } from '../fixtures/vscode';

const binary = process.env.PEGG_TEST_BINARY;

describe.skipIf(!binary)('ChatController against the real ACP server', () => {
  let controller: ChatController;
  let log: Logger;
  let cwd: string;
  let context: any;

  beforeAll(async () => {
    cwd = await mkdtemp(join(tmpdir(), 'pegg-controller-test-'));
    process.env.PEGG_TEST_WORKSPACE = cwd;
    log = new Logger();
    context = {
      subscriptions: [],
      extensionUri: Uri.file(cwd),
      extension: { packageJSON: { version: '0.1.9' } },
      globalStorageUri: Uri.file(join(cwd, 'storage')),
      workspaceState: {
        value: undefined as unknown,
        get() {
          return this.value;
        },
        async update(_key: string, value: unknown) {
          this.value = value;
        },
      },
    };
    controller = new ChatController(context, new BinaryManager(context, log), log);
    await controller.activate();
  });

  afterAll(async () => {
    const peggId = controller.state.peggSessionId;
    await controller.dispose();
    if (peggId) {
      const cleanup = new AcpClient({
        cwd,
        getBinary: async () => binary!,
        log: { debug() {}, info() {}, warn() {} },
      });
      await cleanup.start();
      await cleanup.initialize();
      await cleanup.request('session/delete', { sessionId: peggId }).catch(() => undefined);
      await cleanup.stop();
    }
    log.dispose();
    await rm(cwd, { recursive: true, force: true });
  });

  it('starts a session and lists models', async () => {
    expect(controller.state.connected).toBe(true);
    expect(controller.state.acpSessionId).toBeTruthy();
    expect(controller.state.peggSessionId).toBeTruthy();
    const models = await controller.models();
    expect(Array.isArray(models)).toBe(true);
    if (models.length > 0) {
      await controller.setModel(models[0].provider, models[0].model.id);
      expect(controller.state.provider).toBe(models[0].provider);
      expect(controller.state.model).toBe(models[0].model.id);
    }
  });

  it('changes mode and lists the session', async () => {
    await controller.setMode('plan');
    expect(controller.state.mode).toBe('plan');
    const sessions = await controller.listSessions(false);
    expect(sessions.some((session) => session.id === controller.state.peggSessionId)).toBe(true);
  });

  it('renames the active session', async () => {
    await controller.renameSession(controller.state.peggSessionId!, 'Controller test');
    expect(controller.state.title).toBe('Controller test');
    const sessions = await controller.listSessions(false);
    expect(sessions.find((session) => session.id === controller.state.peggSessionId)?.title).toBe('Controller test');
  });

  it('restarts and resumes the same session', async () => {
    const peggId = controller.state.peggSessionId;
    await controller.restart();
    expect(controller.state.connected).toBe(true);
    expect(controller.state.peggSessionId).toBe(peggId);
  });
});
