import { marked, Renderer } from 'marked';
import hljs from 'highlight.js/lib/common';
import DOMPurify from 'dompurify';

function escapeHtml(value: string): string {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

const renderer = new Renderer();
renderer.code = ({ text, lang }) => {
  const language = (lang ?? '').trim().split(/\s+/)[0].toLowerCase();
  let highlighted: string;
  if (language && hljs.getLanguage(language)) {
    highlighted = hljs.highlight(text, { language, ignoreIllegals: true }).value;
  } else if (!language && text.length < 24000) {
    highlighted = hljs.highlightAuto(text, [
      'go',
      'typescript',
      'javascript',
      'bash',
      'json',
      'python',
      'rust',
      'sql',
      'yaml',
      'css',
      'html',
      'markdown',
    ]).value;
  } else {
    highlighted = escapeHtml(text);
  }
  const label = language || 'code';
  return `<div class="pg-code" data-code-block="true"><div class="pg-code-head"><span class="pg-code-lang">${escapeHtml(
    label,
  )}</span><button class="pg-copy" type="button" data-copy-code="true" aria-label="Copy code">Copy</button></div><pre><code class="hljs${
    language ? ` language-${escapeHtml(language)}` : ''
  }">${highlighted}</code></pre></div>`;
};

renderer.link = function (this: Renderer, { href, title, tokens }) {
  const text = this.parser.parseInline(tokens);
  const safeHref = href ?? '#';
  const titleAttr = title ? ` title="${escapeHtml(title)}"` : '';
  return `<a href="${escapeHtml(safeHref)}"${titleAttr} target="_blank" rel="noreferrer noopener">${text}</a>`;
};

marked.use({
  renderer,
  gfm: true,
  breaks: true,
});

let hookInstalled = false;
function installHook(): void {
  if (hookInstalled) return;
  hookInstalled = true;
  DOMPurify.addHook('afterSanitizeAttributes', (node) => {
    if (node.tagName === 'A') {
      node.setAttribute('target', '_blank');
      node.setAttribute('rel', 'noreferrer noopener');
    }
  });
}

export function renderMarkdown(text: string): string {
  installHook();
  const raw = marked.parse(text ?? '', { async: false }) as string;
  return DOMPurify.sanitize(raw, {
    ADD_ATTR: ['target', 'rel', 'data-copy-code', 'data-code-block'],
    ADD_TAGS: ['button'],
  });
}

export function stripAnsi(text: string): string {
  return text.replace(/\u001b\[[0-9;?]*[ -/]*[@-~]/g, '').replace(/\r/g, '');
}

export function languageForPath(path: string): string {
  const ext = (path.split('.').pop() ?? '').toLowerCase();
  const map: Record<string, string> = {
    go: 'go', ts: 'typescript', tsx: 'typescript', js: 'javascript', jsx: 'javascript',
    py: 'python', rs: 'rust', java: 'java', kt: 'kotlin', kts: 'kotlin', rb: 'ruby',
    php: 'php', swift: 'swift', cs: 'csharp', c: 'c', h: 'c', cpp: 'cpp', hpp: 'cpp',
    sh: 'bash', bash: 'bash', zsh: 'bash', fish: 'bash', sql: 'sql', json: 'json',
    yaml: 'yaml', yml: 'yaml', toml: 'ini', ini: 'ini', css: 'css', scss: 'scss',
    html: 'xml', xml: 'xml', md: 'markdown', dockerfile: 'dockerfile', makefile: 'makefile',
  };
  return map[ext] ?? '';
}

export function highlightCode(code: string, language?: string): string {
  const lang = (language ?? '').toLowerCase();
  if (lang && hljs.getLanguage(lang)) {
    return hljs.highlight(code, { language: lang, ignoreIllegals: true }).value;
  }
  return escapeHtml(code);
}
