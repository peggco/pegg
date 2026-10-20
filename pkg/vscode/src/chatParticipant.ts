import * as vscode from 'vscode';
import { ChatController } from './chat/controller';
import { SessionUpdate } from './acp/types';

export function registerChatParticipant(controller: ChatController): vscode.Disposable {
  const participant = vscode.chat.createChatParticipant('pegg.chat', async (request, _context, stream, token) => {
    const references = await referenceContext(request.references);
    const prompt = buildPrompt(request.command, request.prompt, references);
    const files = new Set<string>();
    let streamed = false;

    const listener = (update: SessionUpdate) => {
      if (token.isCancellationRequested) return;
      switch (update.sessionUpdate) {
        case 'agent_message_chunk': {
          const content = update.content;
          const text =
            typeof content === 'string'
              ? content
              : content && content.type === 'text'
                ? content.text ?? ''
                : '';
          if (text && (!update.agentName || update.agentName === 'orchestrator')) {
            streamed = true;
            stream.markdown(text);
          }
          break;
        }
        case 'tool_call':
          stream.progress(`Pegg: ${update.title ?? 'working'}`);
          for (const location of update.locations ?? []) files.add(location.path);
          break;
        case 'tool_call_update':
          if (update.status === 'failed' && update.error) stream.markdown(`\n\n**Tool failed:** ${update.error}\n\n`);
          break;
        case 'error':
          if (update.error) stream.markdown(`\n\n**Error:** ${update.error}\n\n`);
          break;
      }
    };

    const cancelListener = token.onCancellationRequested(() => void controller.cancel());
    controller.setForceFallback(true);
    try {
      if (controller.state.running) {
        stream.markdown('Pegg is already working on another request. Stop it first or wait for it to finish.');
        return {};
      }
      await controller.activate();
      controller.on('update', listener);
      await controller.sendPrompt(prompt, { includeActiveContext: false });
      for (const file of files) {
        try {
          stream.reference(vscode.Uri.file(file));
        } catch {
          // ignore invalid file references
        }
      }
      if (!streamed) stream.markdown('Done.');
      return {};
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      stream.markdown(`**Pegg error:** ${message}`);
      stream.button({
        command: 'pegg.chat.open',
        title: 'Open Pegg Chat',
      });
      return { errorDetails: { message } };
    } finally {
      controller.off('update', listener);
      controller.setForceFallback(false);
      cancelListener.dispose();
    }
  });

  participant.iconPath = new vscode.ThemeIcon('sparkle');
  return participant;
}

function buildPrompt(command: string | undefined, prompt: string, references: string): string {
  const text = prompt.trim();
  const context = references ? `\n\n${references}` : '';
  switch (command) {
    case 'explain':
      return `Explain the referenced code or question. Cover intent, data flow, and risks.\n\n${text}${context}`;
    case 'fix':
      return `Find the bug or correctness issue and fix it. Make a focused change and verify it.\n\n${text}${context}`;
    case 'review':
      return `/review\n\nReview the referenced code or changes for bugs, regressions, security issues, and missing tests. Report findings as file:line bullets.\n\n${text}${context}`;
    case 'tests':
      return `Write thorough tests for the referenced code using project conventions. Add them and run the relevant test command.\n\n${text}${context}`;
    case 'init':
      return `/init ${text}`.trim();
    case 'rule':
      return `/rule ${text}`.trim();
    case 'batch':
      return `/batch ${text}`.trim();
    default:
      return `${text}${context}`.trim();
  }
}

async function referenceContext(references: readonly vscode.ChatPromptReference[] | undefined): Promise<string> {
  if (!references || references.length === 0) return '';
  const parts: string[] = [];
  for (const reference of references) {
    const value = reference.value;
    try {
      if (value instanceof vscode.Uri) {
        if (value.scheme !== 'file') continue;
        const bytes = await vscode.workspace.fs.readFile(value);
        if (bytes.byteLength > 200_000) continue;
        parts.push(`<file path="${value.fsPath}">\n${Buffer.from(bytes).toString('utf8')}\n</file>`);
      } else if (value instanceof vscode.Location) {
        const doc = await vscode.workspace.openTextDocument(value.uri);
        const text = doc.getText(value.range);
        parts.push(
          `<selection file="${value.uri.fsPath}" lines="${value.range.start.line + 1}-${value.range.end.line + 1}">\n${text}\n</selection>`,
        );
      } else if (typeof value === 'string') {
        parts.push(value);
      }
    } catch {
      // skip references we cannot read
    }
  }
  return parts.length > 0 ? `<context>\n${parts.join('\n\n')}\n</context>` : '';
}
