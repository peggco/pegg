import * as vscode from 'vscode';
import { basename } from 'node:path';
import { getConfig } from '../config';
import { ComposerAttachment } from './state';
import { ContentBlock } from '../acp/types';

export interface ContextInput {
  activeFile?: boolean;
  selection?: vscode.Selection;
  document?: vscode.TextDocument;
}

export function activeContext(input: ContextInput = {}): string {
  const cfg = getConfig();
  const editor = vscode.window.activeTextEditor;
  const doc = input.document ?? editor?.document;
  if (!doc) return '';

  const parts: string[] = [];
  if (cfg.includeActiveFile || input.activeFile) {
    parts.push(`Active file: ${doc.uri.fsPath}`);
  }
  const selection = input.selection ?? editor?.selection;
  if ((cfg.includeSelection || input.selection) && selection && !selection.isEmpty) {
    const text = doc.getText(selection);
    const lang = doc.languageId || '';
    const start = selection.start.line + 1;
    const end = selection.end.line + 1;
    parts.push(`Selected lines ${start}-${end}:\n\`\`\`${lang}\n${text}\n\`\`\``);
  }
  if (parts.length === 0) return '';
  return `<context>\n${parts.join('\n')}\n</context>`;
}

export async function attachmentToBlock(attachment: ComposerAttachment): Promise<ContentBlock | undefined> {
  if (attachment.data && attachment.mediaType.startsWith('image/')) {
    return { type: 'image', data: attachment.data, mimeType: attachment.mediaType };
  }
  if (attachment.data && attachment.mediaType.startsWith('audio/')) {
    return { type: 'audio', data: attachment.data, mimeType: attachment.mediaType };
  }
  if (attachment.data && attachment.mediaType.startsWith('video/')) {
    return { type: 'video', data: attachment.data, mimeType: attachment.mediaType };
  }
  if (attachment.text !== undefined) {
    return {
      type: 'resource',
      resource: {
        uri: attachment.path ?? `untitled:${attachment.name}`,
        mimeType: attachment.mediaType || 'text/plain',
        text: attachment.text,
      },
    };
  }
  if (attachment.path) {
    const bytes = await vscode.workspace.fs.readFile(vscode.Uri.file(attachment.path));
    const cfg = getConfig();
    const limit = bytes.byteLength > 20 * 1024 * 1024 ? 20 * 1024 * 1024 : cfg.maxFileBytes;
    if (bytes.byteLength > limit) {
      return {
        type: 'text',
        text: `[Attached file is too large to inline: ${attachment.path} (${bytes.byteLength} bytes)]`,
      };
    }
    if (attachment.mediaType.startsWith('image/')) {
      return {
        type: 'image',
        data: Buffer.from(bytes).toString('base64'),
        mimeType: attachment.mediaType,
      };
    }
    if (attachment.mediaType.startsWith('audio/')) {
      return {
        type: 'audio',
        data: Buffer.from(bytes).toString('base64'),
        mimeType: attachment.mediaType,
      };
    }
    if (attachment.mediaType.startsWith('video/')) {
      return {
        type: 'video',
        data: Buffer.from(bytes).toString('base64'),
        mimeType: attachment.mediaType,
      };
    }
    if (isBinary(bytes)) {
      return {
        type: 'resource',
        resource: {
          uri: vscode.Uri.file(attachment.path).toString(),
          mimeType: attachment.mediaType || 'application/octet-stream',
          blob: Buffer.from(bytes).toString('base64'),
        },
      };
    }
    return {
      type: 'resource',
      resource: {
        uri: vscode.Uri.file(attachment.path).toString(),
        mimeType: attachment.mediaType || 'text/plain',
        text: Buffer.from(bytes).toString('utf8'),
      },
    };
  }
  return undefined;
}

export async function makeAttachment(path: string): Promise<ComposerAttachment | undefined> {
  try {
    const uri = vscode.Uri.file(path);
    const stat = await vscode.workspace.fs.stat(uri);
    const cfg = getConfig();
    const name = basename(path);
    const mediaType = mediaTypeFor(path);
    if (stat.size > cfg.maxFileBytes && !mediaType.startsWith('image/')) {
      return { id: cryptoId(), name, mediaType, size: stat.size, path };
    }
    const bytes = await vscode.workspace.fs.readFile(uri);
    if (mediaType.startsWith('image/')) {
      return {
        id: cryptoId(),
        name,
        mediaType,
        size: stat.size,
        path,
        data: Buffer.from(bytes).toString('base64'),
      };
    }
    if (isBinary(bytes)) {
      return { id: cryptoId(), name, mediaType, size: stat.size, path };
    }
    return {
      id: cryptoId(),
      name,
      mediaType,
      size: stat.size,
      path,
      text: Buffer.from(bytes).toString('utf8'),
    };
  } catch {
    return undefined;
  }
}

export function makeTextAttachment(name: string, text: string, path?: string): ComposerAttachment {
  return {
    id: cryptoId(),
    name,
    mediaType: 'text/plain',
    size: Buffer.byteLength(text),
    text,
    path,
  };
}

export function isBinary(bytes: Uint8Array): boolean {
  const sample = bytes.subarray(0, Math.min(bytes.length, 8000));
  for (const byte of sample) if (byte === 0) return true;
  return false;
}

export function mediaTypeFor(path: string): string {
  const ext = path.toLowerCase().split('.').pop() ?? '';
  const map: Record<string, string> = {
    png: 'image/png',
    jpg: 'image/jpeg',
    jpeg: 'image/jpeg',
    gif: 'image/gif',
    webp: 'image/webp',
    bmp: 'image/bmp',
    svg: 'image/svg+xml',
    mp3: 'audio/mpeg',
    wav: 'audio/wav',
    ogg: 'audio/ogg',
    m4a: 'audio/mp4',
    mp4: 'video/mp4',
    webm: 'video/webm',
    mov: 'video/quicktime',
    pdf: 'application/pdf',
    json: 'application/json',
    md: 'text/markdown',
    txt: 'text/plain',
    go: 'text/plain',
    ts: 'text/plain',
    tsx: 'text/plain',
    js: 'text/plain',
    jsx: 'text/plain',
    py: 'text/plain',
    rs: 'text/plain',
    java: 'text/plain',
    c: 'text/plain',
    h: 'text/plain',
    cpp: 'text/plain',
    css: 'text/plain',
    html: 'text/plain',
    yaml: 'text/plain',
    yml: 'text/plain',
    toml: 'text/plain',
    sh: 'text/plain',
  };
  return map[ext] ?? 'text/plain';
}

export function cryptoId(): string {
  return `att-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`;
}
