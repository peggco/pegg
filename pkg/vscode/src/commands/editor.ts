import * as vscode from 'vscode';
import { ChatController } from '../chat/controller';
import { activeContext, makeAttachment, makeTextAttachment } from '../chat/context';
import { getConfig } from '../config';
import { gitDiff, relativePath } from '../integrations/git';

interface ActionItem extends vscode.QuickPickItem {
  run: () => Thenable<void> | Promise<void> | void;
}

type ResourceArg = string | vscode.Uri | undefined;

export function registerEditorCommands(controller: ChatController): vscode.Disposable[] {
  return [
    vscode.commands.registerCommand('pegg.editorActions', () => editorActions(controller)),
    vscode.commands.registerCommand('pegg.selectionActions', () => selectionActions(controller)),
    vscode.commands.registerCommand('pegg.fileActions', (uri?: ResourceArg) => fileActions(uri)),
    vscode.commands.registerCommand('pegg.explainSelection', (uri?: string, range?: vscode.Range) =>
      selectionPrompt(controller, 'Explain this code. Focus on intent, data flow, and any risks.', uri, range),
    ),
    vscode.commands.registerCommand('pegg.fixSelection', (uri?: string, range?: vscode.Range) =>
      selectionPrompt(
        controller,
        'Find bugs or correctness issues in this code, then fix them. Keep the change focused and explain what you changed.',
        uri,
        range,
      ),
    ),
    vscode.commands.registerCommand('pegg.reviewSelection', (uri?: string, range?: vscode.Range) =>
      selectionPrompt(
        controller,
        'Review this code for bugs, security issues, performance problems, and missing tests. Report findings as file:line bullets.',
        uri,
        range,
      ),
    ),
    vscode.commands.registerCommand('pegg.findBugsSelection', (uri?: string, range?: vscode.Range) =>
      selectionPrompt(
        controller,
        'Find bugs and correctness issues in this code. Report each finding as file:line with severity, impact, and a concrete fix.',
        uri,
        range,
      ),
    ),
    vscode.commands.registerCommand('pegg.securityReviewSelection', (uri?: string, range?: vscode.Range) =>
      selectionPrompt(
        controller,
        'Review this code for security vulnerabilities. Report severity, file:line, exploit path, and a concrete fix.',
        uri,
        range,
      ),
    ),
    vscode.commands.registerCommand('pegg.generateTests', (uri?: string, range?: vscode.Range) =>
      selectionPrompt(
        controller,
        'Write thorough tests for this code using the project conventions. Add the tests to the appropriate test file and run them.',
        uri,
        range,
      ),
    ),
    vscode.commands.registerCommand('pegg.refactorSelection', (uri?: string, range?: vscode.Range) =>
      selectionPrompt(
        controller,
        'Refactor this code for clarity and maintainability without changing behavior. Preserve public APIs unless necessary.',
        uri,
        range,
      ),
    ),
    vscode.commands.registerCommand('pegg.optimizeSelection', (uri?: string, range?: vscode.Range) =>
      selectionPrompt(
        controller,
        'Optimize this code for performance and clarity without changing behavior. Explain the tradeoffs briefly.',
        uri,
        range,
      ),
    ),
    vscode.commands.registerCommand('pegg.documentSelection', (uri?: string, range?: vscode.Range) =>
      selectionPrompt(
        controller,
        "Add or improve documentation comments for this code. Match the project's style and do not change behavior.",
        uri,
        range,
      ),
    ),
    vscode.commands.registerCommand('pegg.askSelection', async (uri?: string, range?: vscode.Range) => {
      const doc = await resolveDocument(uri);
      if (!doc) return;
      const selection = effectiveSelection(doc, range);
      const question = await vscode.window.showInputBox({
        title: 'Ask Pegg about selection',
        prompt: 'What do you want to know about the selected code?',
        placeHolder: 'e.g. Why is this slow? What are the edge cases?',
        ignoreFocusOut: true,
      });
      if (!question?.trim()) return;
      await run(controller, question.trim(), doc, selection);
    }),
    vscode.commands.registerCommand('pegg.editSelection', async (uri?: string, range?: vscode.Range) => {
      const doc = await resolveDocument(uri);
      if (!doc) return;
      const selection = effectiveSelection(doc, range);
      const instruction = await vscode.window.showInputBox({
        title: 'Edit selection with Pegg',
        prompt: 'Describe the change you want.',
        placeHolder: 'e.g. Handle the nil case and add a test',
        ignoreFocusOut: true,
      });
      if (!instruction?.trim()) return;
      await run(
        controller,
        `Edit the selected code according to this instruction. Make the change in place, keep the diff focused, and verify it.\n\nInstruction: ${instruction.trim()}`,
        doc,
        selection,
      );
    }),
    vscode.commands.registerCommand('pegg.addSelectionToChat', async (uri?: string, range?: vscode.Range) => {
      const doc = await resolveDocument(uri);
      if (!doc) return;
      const selection = range ?? vscode.window.activeTextEditor?.selection;
      const target = selection && !selection.isEmpty ? selection : effectiveSelection(doc, range);
      const text = doc.getText(target);
      const label = `${relativePath(doc.uri.fsPath)}:${target.start.line + 1}-${target.end.line + 1}`;
      controller.addComposerAttachment(makeTextAttachment(label, text, doc.uri.fsPath));
      await vscode.commands.executeCommand('pegg.chat.open');
    }),
    vscode.commands.registerCommand('pegg.addFileToChat', async (uri?: ResourceArg) => {
      const doc = await resolveDocument(uri);
      if (!doc || doc.uri.scheme !== 'file') return;
      const attachment = await makeAttachment(doc.uri.fsPath);
      if (attachment) controller.addComposerAttachment(attachment);
      await vscode.commands.executeCommand('pegg.chat.open');
    }),
    vscode.commands.registerCommand('pegg.explainFile', async (uri?: ResourceArg) => {
      const doc = await resolveDocument(uri);
      if (!doc) return;
      const attachment = doc.uri.scheme === 'file' ? await makeAttachment(doc.uri.fsPath) : undefined;
      if (attachment) controller.addComposerAttachment(attachment);
      await vscode.commands.executeCommand('pegg.chat.open');
      await controller.sendPrompt(
        `Explain ${relativePath(doc.uri.fsPath)}. Cover its purpose, structure, important functions, and anything surprising.`,
        { attachments: attachment ? [attachment] : [], includeActiveContext: false },
      );
    }),
    vscode.commands.registerCommand('pegg.askAboutFile', async (uri?: ResourceArg) => {
      const doc = await resolveDocument(uri);
      if (!doc) return;
      const question = await vscode.window.showInputBox({
        title: 'Ask Pegg about this file',
        prompt: `What do you want to know about ${relativePath(doc.uri.fsPath)}?`,
        placeHolder: 'e.g. Where is this initialized?',
        ignoreFocusOut: true,
      });
      if (!question?.trim()) return;
      await runFile(controller, question.trim(), doc);
    }),
    vscode.commands.registerCommand('pegg.editFile', async (uri?: ResourceArg) => {
      const doc = await resolveDocument(uri);
      if (!doc) return;
      const instruction = await vscode.window.showInputBox({
        title: 'Edit file with Pegg',
        prompt: `Describe the change for ${relativePath(doc.uri.fsPath)}.`,
        placeHolder: 'e.g. Add input validation and tests',
        ignoreFocusOut: true,
      });
      if (!instruction?.trim()) return;
      await runFile(
        controller,
        `Edit ${relativePath(doc.uri.fsPath)} according to this instruction. Keep the diff focused and verify it.\n\nInstruction: ${instruction.trim()}`,
        doc,
      );
    }),
    vscode.commands.registerCommand('pegg.reviewFile', async (uri?: ResourceArg) => {
      const doc = await resolveDocument(uri);
      if (!doc) return;
      const rel = relativePath(doc.uri.fsPath);
      let text = '';
      if (doc.uri.scheme === 'file') {
        try {
          text = (await gitDiff(rel)).text;
        } catch {
          text = '';
        }
      }
      const body = text || fileSnippet(doc);
      await vscode.commands.executeCommand('pegg.chat.open');
      await controller.sendPrompt(
        `/review\n\nReview ${rel} for bugs, regressions, security issues, performance problems, and missing tests. Report findings as file:line bullets.\n\n${body}`,
        { includeActiveContext: false },
      );
    }),
    vscode.commands.registerCommand('pegg.documentFile', async (uri?: ResourceArg) => {
      const doc = await resolveDocument(uri);
      if (!doc) return;
      await runFile(
        controller,
        `Add or improve documentation comments across ${relativePath(doc.uri.fsPath)}. Match the project's style and do not change behavior.`,
        doc,
      );
    }),
    vscode.commands.registerCommand('pegg.generateTestsForFile', async (uri?: ResourceArg) => {
      const doc = await resolveDocument(uri);
      if (!doc) return;
      await runFile(
        controller,
        `Write thorough tests for ${relativePath(doc.uri.fsPath)} using the project conventions. Add the tests to the appropriate test file and run them.`,
        doc,
      );
    }),
    vscode.commands.registerCommand('pegg.fixDiagnosticsInFile', async (uri?: ResourceArg) => {
      const doc = await resolveDocument(uri);
      if (!doc) return;
      const diagnostics = vscode.languages
        .getDiagnostics(doc.uri)
        .filter(
          (diagnostic) =>
            diagnostic.severity === vscode.DiagnosticSeverity.Error ||
            diagnostic.severity === vscode.DiagnosticSeverity.Warning,
        )
        .slice(0, 100);
      if (diagnostics.length === 0) {
        void vscode.window.showInformationMessage('No errors or warnings in this file.');
        return;
      }
      const list = diagnostics
        .map((diagnostic) => {
          const severity = diagnostic.severity === vscode.DiagnosticSeverity.Error ? 'error' : 'warning';
          const line = diagnostic.range.start.line + 1;
          return `- ${severity} line ${line}: ${diagnostic.message}${diagnostic.source ? ` (${diagnostic.source})` : ''}`;
        })
        .join('\n');
      await runFile(
        controller,
        `Fix all diagnostics in ${relativePath(doc.uri.fsPath)}. Make focused changes, preserve behavior, and verify with the relevant checks.\n\nDiagnostics:\n${list}`,
        doc,
      );
    }),
    vscode.commands.registerCommand(
      'pegg.explainDiagnostic',
      async (uri: string, range: vscode.Range, message: string) => {
        const doc = await resolveDocument(uri);
        if (!doc) return;
        await run(controller, `Explain this diagnostic:\n\n${message}\n\n${selectionSnippet(doc, range)}`, doc);
      },
    ),
    vscode.commands.registerCommand(
      'pegg.fixDiagnostic',
      async (uri: string, range: vscode.Range, message: string) => {
        const doc = await resolveDocument(uri);
        if (!doc) return;
        await run(
          controller,
          `Fix the following diagnostic in ${relativePath(doc.uri.fsPath)}:\n\n${message}\n\n${selectionSnippet(doc, range)}\n\nMake the smallest correct change and verify it.`,
          doc,
        );
      },
    ),
    vscode.commands.registerCommand('pegg.reviewChanges', async (staged = false) => {
      const result = await gitDiff(undefined, staged === true);
      if (!result.text) {
        void vscode.window.showInformationMessage('No changes to review.');
        return;
      }
      await vscode.commands.executeCommand('pegg.chat.open');
      await controller.sendPrompt(
        `/review\n\nReview these changes for bugs, regressions, security issues, and missing tests. Report findings as file:line bullets.\n\n${result.text}`,
        { includeActiveContext: false },
      );
    }),
    vscode.commands.registerCommand('pegg.reviewCurrentFile', async () => {
      const doc = vscode.window.activeTextEditor?.document;
      if (!doc || doc.uri.scheme !== 'file') return;
      await vscode.commands.executeCommand('pegg.reviewFile', doc.uri);
    }),
  ];
}

async function editorActions(controller: ChatController): Promise<void> {
  const editor = vscode.window.activeTextEditor;
  if (!editor) {
    await fileActions(undefined);
    return;
  }
  if (editor.selection.isEmpty) await fileActions(editor.document.uri);
  else await selectionActions(controller);
}

async function selectionActions(controller: ChatController): Promise<void> {
  const editor = vscode.window.activeTextEditor;
  if (!editor || editor.selection.isEmpty) {
    await fileActions(editor?.document.uri);
    return;
  }
  const uri = editor.document.uri.toString();
  const range = editor.selection;
  const items: ActionItem[] = [
    {
      label: '$(comment-discussion) Explain selection',
      description: 'Understand intent, data flow, and risks',
      run: () => selectionPrompt(controller, 'Explain this code. Focus on intent, data flow, and any risks.', uri, range),
    },
    {
      label: '$(edit) Edit selection…',
      description: 'Describe a change and let Pegg apply it',
      run: () => vscode.commands.executeCommand('pegg.editSelection', uri, range),
    },
    {
      label: '$(question) Ask about selection…',
      description: 'Ask a free-form question',
      run: () => vscode.commands.executeCommand('pegg.askSelection', uri, range),
    },
    {
      label: '$(bug) Find bugs',
      description: 'Correctness review with concrete fixes',
      run: () => vscode.commands.executeCommand('pegg.findBugsSelection', uri, range),
    },
    {
      label: '$(shield) Security review',
      description: 'Look for vulnerabilities and unsafe input handling',
      run: () => vscode.commands.executeCommand('pegg.securityReviewSelection', uri, range),
    },
    {
      label: '$(symbol-class) Refactor',
      description: 'Improve structure without changing behavior',
      run: () => vscode.commands.executeCommand('pegg.refactorSelection', uri, range),
    },
    {
      label: '$(dashboard) Optimize',
      description: 'Improve performance and clarity',
      run: () => vscode.commands.executeCommand('pegg.optimizeSelection', uri, range),
    },
    {
      label: '$(book) Add documentation',
      description: 'Add or improve doc comments',
      run: () => vscode.commands.executeCommand('pegg.documentSelection', uri, range),
    },
    {
      label: '$(beaker) Generate tests',
      description: 'Write tests and run them',
      run: () => vscode.commands.executeCommand('pegg.generateTests', uri, range),
    },
    {
      label: '$(comment-add) Add selection to chat',
      description: 'Keep the selection as composer context',
      run: () => vscode.commands.executeCommand('pegg.addSelectionToChat', uri, range),
    },
  ];
  const picked = await vscode.window.showQuickPick(items, {
    title: 'Pegg: Selection Actions',
    placeHolder: 'Choose an AI action for the selected code',
    matchOnDescription: true,
  });
  await picked?.run();
}

async function fileActions(uri?: ResourceArg): Promise<void> {
  const doc = await resolveDocument(uri);
  if (!doc) return;
  const resource = doc.uri.toString();
  const items: ActionItem[] = [
    {
      label: '$(comment-discussion) Explain file',
      description: 'Purpose, structure, and important functions',
      run: () => vscode.commands.executeCommand('pegg.explainFile', resource),
    },
    {
      label: '$(edit) Edit file…',
      description: 'Describe a change and let Pegg apply it',
      run: () => vscode.commands.executeCommand('pegg.editFile', resource),
    },
    {
      label: '$(question) Ask about file…',
      description: 'Ask a free-form question',
      run: () => vscode.commands.executeCommand('pegg.askAboutFile', resource),
    },
    {
      label: '$(git-compare) Review file',
      description: 'Review the working-tree diff or the whole file',
      run: () => vscode.commands.executeCommand('pegg.reviewFile', resource),
    },
    {
      label: '$(book) Document file',
      description: 'Add or improve doc comments',
      run: () => vscode.commands.executeCommand('pegg.documentFile', resource),
    },
    {
      label: '$(beaker) Generate tests for file',
      description: 'Write tests and run them',
      run: () => vscode.commands.executeCommand('pegg.generateTestsForFile', resource),
    },
    {
      label: '$(warning) Fix all diagnostics',
      description: 'Fix errors and warnings in this file',
      run: () => vscode.commands.executeCommand('pegg.fixDiagnosticsInFile', resource),
    },
    {
      label: '$(comment-add) Add file to chat',
      description: 'Attach the file to the composer',
      run: () => vscode.commands.executeCommand('pegg.addFileToChat', resource),
    },
  ];
  const picked = await vscode.window.showQuickPick(items, {
    title: `Pegg: File Actions — ${relativePath(doc.uri.fsPath)}`,
    placeHolder: 'Choose an AI action for this file',
    matchOnDescription: true,
  });
  await picked?.run();
}

async function selectionPrompt(
  controller: ChatController,
  instruction: string,
  uri?: string,
  range?: vscode.Range,
): Promise<void> {
  const doc = await resolveDocument(uri);
  if (!doc) {
    await vscode.commands.executeCommand('pegg.chat.open');
    return;
  }
  await run(controller, instruction, doc, effectiveSelection(doc, range));
}

async function run(
  controller: ChatController,
  instruction: string,
  doc: vscode.TextDocument,
  selection?: vscode.Selection | vscode.Range,
): Promise<void> {
  const cfg = getConfig();
  const snippet = selection && !selection.isEmpty ? doc.getText(selection) : '';
  const context = [
    `File: ${relativePath(doc.uri.fsPath)}`,
    snippet
      ? `Lines ${selection!.start.line + 1}-${selection!.end.line + 1}:\n\`\`\`${doc.languageId}\n${snippet}\n\`\`\``
      : '',
    cfg.includeActiveFile && !snippet ? activeContext() : '',
  ]
    .filter(Boolean)
    .join('\n\n');
  await vscode.commands.executeCommand('pegg.chat.open');
  await controller.sendPrompt(`${instruction}\n\n${context}`, { includeActiveContext: false });
}

async function runFile(controller: ChatController, instruction: string, doc: vscode.TextDocument): Promise<void> {
  const context = [`File: ${relativePath(doc.uri.fsPath)}`, fileSnippet(doc)].filter(Boolean).join('\n\n');
  await vscode.commands.executeCommand('pegg.chat.open');
  await controller.sendPrompt(`${instruction}\n\n${context}`, { includeActiveContext: false });
}

function fileSnippet(doc: vscode.TextDocument): string {
  const text = doc.getText();
  if (text.length > 128 * 1024) return '';
  return `\`\`\`${doc.languageId}\n${text}\n\`\`\``;
}

function effectiveSelection(
  doc: vscode.TextDocument,
  range?: vscode.Range,
): vscode.Selection | vscode.Range {
  const active = vscode.window.activeTextEditor?.document.uri.toString() === doc.uri.toString()
    ? vscode.window.activeTextEditor?.selection
    : undefined;
  if (range && !range.isEmpty) return range;
  if (active && !active.isEmpty) return active;
  const line = new vscode.Range(new vscode.Position(0, 0), new vscode.Position(0, 0));
  const position = vscode.window.activeTextEditor?.selection.active ?? line.start;
  const lineNumber = Math.min(Math.max(position.line, 0), doc.lineCount - 1);
  const text = doc.lineAt(lineNumber).text;
  return new vscode.Range(lineNumber, 0, lineNumber, text.length);
}

async function resolveDocument(uri?: ResourceArg): Promise<vscode.TextDocument | undefined> {
  if (!uri) return vscode.window.activeTextEditor?.document;
  const parsed = typeof uri === 'string' ? vscode.Uri.parse(uri) : uri;
  const open = vscode.workspace.textDocuments.find((doc) => doc.uri.toString() === parsed.toString());
  if (open) return open;
  try {
    return await vscode.workspace.openTextDocument(parsed);
  } catch {
    return undefined;
  }
}

function selectionSnippet(doc: vscode.TextDocument, range: vscode.Range): string {
  const start = Math.max(0, range.start.line - 3);
  const end = Math.min(doc.lineCount - 1, range.end.line + 3);
  const text = doc.getText(new vscode.Range(start, 0, end, doc.lineAt(end).text.length));
  return `\`\`\`${doc.languageId}\n${text}\n\`\`\``;
}
