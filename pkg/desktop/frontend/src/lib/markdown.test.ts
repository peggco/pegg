import { describe, expect, it } from 'vitest';
import { renderMarkdown } from './markdown';

describe('markdown rendering', () => {
  it('renders fenced code with a copy control', () => {
    const html = renderMarkdown('```go\nfmt.Println("hi")\n```');
    expect(html).toContain('data-copy-code');
    expect(html).toContain('hljs');
    expect(html).toContain('Println');
  });

  it('sanitizes scripts', () => {
    const html = renderMarkdown('<script>alert(1)</script><p>safe</p>');
    expect(html).not.toContain('<script');
    expect(html).toContain('safe');
  });
});
