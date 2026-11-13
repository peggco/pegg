import { render } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { ToolCard } from './ToolViews';
import type { ToolState } from '../state/chat';

function tool(overrides: Partial<ToolState>): ToolState {
  return {
    id: 't1',
    title: 'Tool',
    status: 'completed',
    ...overrides,
  };
}

describe('ToolCard', () => {
  it('keeps read outputs collapsed', () => {
    const { container } = render(
      <ToolCard tool={tool({ name: 'read', kind: 'read', title: 'Read main.go', output: 'SECRET-OUTPUT' })} />,
    );
    expect(container.textContent).toContain('Read main.go');
    expect(container.textContent).not.toContain('SECRET-OUTPUT');
    expect(container.querySelector('.tool-chevron')).toBeNull();
  });

  it('renders a todo write as an inline plan card', () => {
    const { container } = render(
      <ToolCard
        tool={tool({
          name: 'todowrite',
          title: 'todowrite',
          input: {
            todos: [
              { title: 'Inspect', status: 'completed' },
              { title: 'Implement', status: 'in_progress' },
            ],
          },
        })}
      />,
    );
    expect(container.querySelector('.plan-card')).not.toBeNull();
    expect(container.textContent).toContain('Inspect');
    expect(container.textContent).toContain('Implement');
    expect(container.querySelector('.tool-mini')).toBeNull();
  });

  it('renders bash as a terminal', () => {
    const { container } = render(
      <ToolCard
        tool={tool({
          name: 'bash',
          kind: 'execute',
          title: 'bash: go test ./...',
          input: { command: 'go test ./...' },
          output: 'ok  github.com/peggco/pegg/internal/acp\n',
        })}
      />,
    );
    expect(container.querySelector('.terminal-card')).not.toBeNull();
    expect(container.textContent).toContain('$');
    expect(container.textContent).toContain('go test ./...');
    expect(container.textContent).toContain('ok  github.com/peggco/pegg/internal/acp');
  });

  it('shows highlighted write content', () => {
    const { container } = render(
      <ToolCard
        tool={tool({
          name: 'write',
          kind: 'edit',
          title: 'Write demo.go',
          input: { path: 'demo.go', content: 'package demo\n\nfunc main() {}\n' },
          diff: { path: 'demo.go', oldText: '', newText: 'package demo\n\nfunc main() {}\n' },
        })}
      />,
    );
    expect(container.querySelector('.write-card')).not.toBeNull();
    expect(container.textContent).toContain('package demo');
    expect(container.querySelector('.hljs-keyword')).not.toBeNull();
  });

  it('renders a line-numbered diff for edits', () => {
    const { container } = render(
      <ToolCard
        tool={tool({
          name: 'edit',
          kind: 'edit',
          title: 'Edit demo.go',
          diff: {
            path: 'demo.go',
            oldText: 'package demo\n\nfunc main() { println("old") }\n',
            newText: 'package demo\n\nfunc main() { println("new") }\n',
          },
        })}
      />,
    );
    expect(container.querySelector('.gh-diff')).not.toBeNull();
    expect(container.querySelector('.diff-hunk')?.textContent).toContain('@@');
    expect(container.textContent).toContain('println("new")');
    expect(container.querySelectorAll('.diff-num').length).toBeGreaterThan(0);
  });

  it('expands errors for compact tools', () => {
    const { container } = render(
      <ToolCard
        tool={tool({ name: 'read', kind: 'read', title: 'Read main.go', status: 'failed', error: 'permission denied' })}
      />,
    );
    expect(container.textContent).toContain('Show error');
    expect(container.textContent).not.toContain('permission denied');
  });

  it('renders delivered files with open and save actions', () => {
    const { container } = render(
      <ToolCard
        tool={tool({
          name: 'sendfile',
          title: 'sendfile:2 files',
          files: [
            { name: 'report.pdf', mediaType: 'application/pdf', size: 1536, path: '/tmp/report.pdf' },
            { name: 'app.zip', mediaType: 'application/zip', size: 5 << 20, path: '/tmp/app.zip' },
          ],
        })}
      />,
    );
    expect(container.querySelector('.files-card')).not.toBeNull();
    expect(container.querySelectorAll('.files-row')).toHaveLength(2);
    expect(container.textContent).toContain('report.pdf');
    expect(container.textContent).toContain('1.5 KB');
    expect(container.textContent).toContain('5.0 MB');
    expect(container.textContent).toContain('Save a copy');
  });
});
