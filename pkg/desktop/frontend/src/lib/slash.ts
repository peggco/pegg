export interface SlashToken {
  start: number;
  query: string;
}

const SLASH_TOKEN = /(^|\s)\/([^\s/]*)$/;

export function slashToken(text: string): SlashToken | null {
  const match = SLASH_TOKEN.exec(text);
  if (!match) return null;
  return { start: match.index + match[1].length, query: match[2] };
}
