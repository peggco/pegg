import * as vscode from 'vscode';

export function registerCodeActions(): vscode.Disposable {
  return vscode.languages.registerCodeActionsProvider(
    [{ scheme: 'file' }, { scheme: 'untitled' }],
    new PeggCodeActionProvider(),
    { providedCodeActionKinds: [vscode.CodeActionKind.QuickFix, vscode.CodeActionKind.RefactorRewrite] },
  );
}

class PeggCodeActionProvider implements vscode.CodeActionProvider {
  provideCodeActions(
    document: vscode.TextDocument,
    range: vscode.Range,
    context: vscode.CodeActionContext,
  ): vscode.ProviderResult<vscode.CodeAction[]> {
    const actions: vscode.CodeAction[] = [];
    const selection = !range.isEmpty
      ? document.getText(range)
      : context.diagnostics[0]
        ? document.getText(context.diagnostics[0].range)
        : '';

    for (const diagnostic of context.diagnostics.slice(0, 3)) {
      const fix = new vscode.CodeAction(
        `Fix with Pegg: ${truncate(diagnostic.message, 60)}`,
        vscode.CodeActionKind.QuickFix,
      );
      fix.diagnostics = [diagnostic];
      fix.command = {
        command: 'pegg.fixDiagnostic',
        title: 'Fix with Pegg',
        arguments: [document.uri.toString(), diagnostic.range, diagnostic.message],
      };
      actions.push(fix);

      const explain = new vscode.CodeAction('Explain with Pegg', vscode.CodeActionKind.QuickFix);
      explain.diagnostics = [diagnostic];
      explain.command = {
        command: 'pegg.explainDiagnostic',
        title: 'Explain with Pegg',
        arguments: [document.uri.toString(), diagnostic.range, diagnostic.message],
      };
      actions.push(explain);
    }

    if (context.diagnostics.length > 1) {
      const fixAll = new vscode.CodeAction(
        `Fix all ${context.diagnostics.length} diagnostics with Pegg`,
        vscode.CodeActionKind.QuickFix,
      );
      fixAll.command = {
        command: 'pegg.fixDiagnosticsInFile',
        title: 'Fix all diagnostics with Pegg',
        arguments: [document.uri.toString()],
      };
      actions.push(fixAll);
    }

    if (selection.trim()) {
      const items: Array<[string, vscode.CodeActionKind, string]> = [
        ['Explain selection with Pegg', vscode.CodeActionKind.RefactorRewrite, 'pegg.explainSelection'],
        ['Edit selection with Pegg…', vscode.CodeActionKind.RefactorRewrite, 'pegg.editSelection'],
        ['Ask Pegg about selection…', vscode.CodeActionKind.RefactorRewrite, 'pegg.askSelection'],
        ['Fix selection with Pegg', vscode.CodeActionKind.QuickFix, 'pegg.fixSelection'],
        ['Find bugs with Pegg', vscode.CodeActionKind.QuickFix, 'pegg.findBugsSelection'],
        ['Review selection with Pegg', vscode.CodeActionKind.RefactorRewrite, 'pegg.reviewSelection'],
        ['Security review with Pegg', vscode.CodeActionKind.RefactorRewrite, 'pegg.securityReviewSelection'],
        ['Refactor selection with Pegg', vscode.CodeActionKind.RefactorRewrite, 'pegg.refactorSelection'],
        ['Optimize selection with Pegg', vscode.CodeActionKind.RefactorRewrite, 'pegg.optimizeSelection'],
        ['Document selection with Pegg', vscode.CodeActionKind.RefactorRewrite, 'pegg.documentSelection'],
        ['Add tests with Pegg', vscode.CodeActionKind.RefactorRewrite, 'pegg.generateTests'],
        ['Add selection to Pegg chat', vscode.CodeActionKind.RefactorRewrite, 'pegg.addSelectionToChat'],
      ];
      for (const [title, kind, command] of items) {
        const action = new vscode.CodeAction(title, kind);
        action.command = { command, title, arguments: [document.uri.toString(), range] };
        actions.push(action);
      }
    }
    return actions;
  }
}

function truncate(value: string, max: number): string {
  return value.length > max ? `${value.slice(0, max - 1)}…` : value;
}
