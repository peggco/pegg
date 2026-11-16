import { describe, expect, it } from 'vitest';
import { branchNameFor, renderBranchTemplate, slugify } from '../../src/integrations/branchName';

describe('slugify', () => {
  it('lowercases and dashes titles', () => {
    expect(slugify('Fix Login Redirect')).toBe('fix-login-redirect');
  });

  it('drops leading bracketed tags and parentheticals', () => {
    expect(slugify('[FEATURE] Fix login (again)')).toBe('fix-login');
    expect(slugify('[BUG]: Crash on start')).toBe('crash-on-start');
  });

  it('collapses separators and trims dashes', () => {
    expect(slugify('  Foo / Bar -- Baz! ')).toBe('foo-bar-baz');
  });

  it('caps long titles on a word boundary', () => {
    const slug = slugify('A very long issue title that keeps going and going and going forever');
    expect(slug.length).toBeLessThanOrEqual(40);
    expect(slug.startsWith('a-very-long-issue-title')).toBe(true);
    expect(slug.endsWith('-')).toBe(false);
  });
});

describe('renderBranchTemplate', () => {
  it('renders the default number and slug template', () => {
    expect(renderBranchTemplate('{number}-{slug}', { number: 42, title: 'Fix Login' })).toBe(
      '42-fix-login',
    );
  });

  it('uses iid when number is missing', () => {
    expect(renderBranchTemplate('{number}-{slug}', { iid: 7, title: 'MR flow' })).toBe('7-mr-flow');
  });

  it('supports explicit prefixes and the provider variable', () => {
    expect(
      renderBranchTemplate('{provider}/{number}-{slug}', {
        number: 1,
        title: 'Hello',
        provider: 'github',
      }),
    ).toBe('github/1-hello');
    expect(renderBranchTemplate('issue/{number}', { number: 9 })).toBe('issue/9');
  });

  it('omits an unavailable number without leaving separators', () => {
    expect(renderBranchTemplate('{number}-{slug}', { title: 'No issue' })).toBe('no-issue');
    expect(renderBranchTemplate('{number}/{slug}', { title: 'No issue' })).toBe('no-issue');
  });

  it('sanitizes unsafe characters and invalid separators', () => {
    expect(renderBranchTemplate('{number} {slug}', { number: 3, title: 'Fix: login?' })).toBe(
      '3-fix-login',
    );
    expect(renderBranchTemplate('{slug}', { title: '   ' })).toBe('');
  });
});

describe('branchNameFor', () => {
  it('falls back to the title when the template renders empty', () => {
    expect(branchNameFor('{number}', { title: 'Fallback title' })).toBe('fallback-title');
    expect(branchNameFor('{number}', {})).toBe('work');
  });
});
