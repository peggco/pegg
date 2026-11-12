import { Marked } from 'marked';
import hljs from 'highlight.js/lib/common';
import DOMPurify from 'dompurify';

const marked = new Marked({
  gfm: true,
  breaks: true,
  renderer: {
    code({ text, lang }) {
      const language = (lang || '').split(/\s+/)[0];
      let highlighted: string;
      try {
        highlighted = language && hljs.getLanguage(language)
          ? hljs.highlight(text, { language }).value
          : hljs.highlightAuto(text).value;
      } catch {
        highlighted = escapeHtml(text);
      }
      const label = language ? escapeHtml(language) : 'text';
      return `<div class="code-block"><div class="code-label">${label}</div><pre><code class="hljs">${highlighted}</code></pre></div>`;
    },
    link({ href, title, tokens }) {
      const text = this.parser.parseInline(tokens);
      const safe = escapeAttr(href ?? '');
      const titleAttr = title ? ` title="${escapeAttr(title)}"` : '';
      return `<a href="${safe}"${titleAttr} data-href="${safe}">${text}</a>`;
    },
  },
});

export function renderMarkdown(text: string): string {
  const html = marked.parse(text ?? '') as string;
  return DOMPurify.sanitize(html, {
    ADD_ATTR: ['data-href', 'target', 'rel'],
    FORBID_TAGS: ['style', 'form', 'input', 'button'],
  });
}

export function languageForPath(path: string): string {
  const ext = (path.split('.').pop() ?? '').toLowerCase();
  const map: Record<string, string> = {
    go: 'go',
    ts: 'typescript',
    tsx: 'typescript',
    js: 'javascript',
    jsx: 'javascript',
    py: 'python',
    rs: 'rust',
    java: 'java',
    kt: 'kotlin',
    kts: 'kotlin',
    rb: 'ruby',
    php: 'php',
    swift: 'swift',
    cs: 'csharp',
    c: 'c',
    h: 'c',
    cpp: 'cpp',
    hpp: 'cpp',
    sh: 'bash',
    bash: 'bash',
    zsh: 'bash',
    fish: 'bash',
    sql: 'sql',
    json: 'json',
    yaml: 'yaml',
    yml: 'yaml',
    toml: 'ini',
    ini: 'ini',
    css: 'css',
    scss: 'scss',
    html: 'xml',
    xml: 'xml',
    md: 'markdown',
    dockerfile: 'dockerfile',
    makefile: 'makefile',
  };
  return map[ext] ?? '';
}

export function highlightCode(code: string, language?: string): string {
  const lang = (language ?? '').toLowerCase();
  try {
    if (lang && hljs.getLanguage(lang)) {
      return hljs.highlight(code, { language: lang, ignoreIllegals: true }).value;
    }
    return hljs.highlightAuto(code).value;
  } catch {
    return escapeHtml(code);
  }
}

function escapeHtml(value: string): string {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

function escapeAttr(value: string): string {
  return escapeHtml(value).replace(/'/g, '&#39;');
}
