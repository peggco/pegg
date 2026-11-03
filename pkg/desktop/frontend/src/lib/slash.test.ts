import { describe, expect, it } from 'vitest';
import { slashToken } from './slash';

describe('slashToken', () => {
  it('detects a token at the start of the text', () => {
    expect(slashToken('/rem')).toEqual({ start: 0, query: 'rem' });
  });

  it('detects a bare slash with no query', () => {
    expect(slashToken('/')).toEqual({ start: 0, query: '' });
  });

  it('detects a token after whitespace', () => {
    expect(slashToken('make a video /rem')).toEqual({ start: 13, query: 'rem' });
  });

  it('detects a token after a newline', () => {
    expect(slashToken('line one\n/r')).toEqual({ start: 9, query: 'r' });
  });

  it('closes the token once a space is typed', () => {
    expect(slashToken('/remotion ')).toBeNull();
    expect(slashToken('/review text')).toBeNull();
  });

  it('ignores paths and mid-word slashes', () => {
    expect(slashToken('src/main.go')).toBeNull();
    expect(slashToken('see foo/bar')).toBeNull();
  });

  it('ignores plain text', () => {
    expect(slashToken('')).toBeNull();
    expect(slashToken('hello world')).toBeNull();
  });
});
