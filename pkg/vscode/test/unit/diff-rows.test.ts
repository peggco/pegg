import { describe, expect, it } from 'vitest';
import { computeDiffRows } from '../../../chat-ui/src/webview/diff';

describe('computeDiffRows', () => {
  it('returns no rows for identical text', () => {
    expect(computeDiffRows('same\ntext', 'same\ntext')).toEqual([]);
    expect(computeDiffRows('', '')).toEqual([]);
  });

  it('marks a single added line with the new line number', () => {
    const rows = computeDiffRows('a\nb', 'a\nb\nc');
    const hunk = rows.find((row) => row.kind === 'hunk');
    expect(hunk?.text).toBe('@@ -1,2 +1,3 @@');
    const added = rows.filter((row) => row.kind === 'add');
    expect(added).toHaveLength(1);
    expect(added[0]).toMatchObject({ text: 'c', newLine: 3 });
  });

  it('marks a single removed line with the old line number', () => {
    const rows = computeDiffRows('a\nb\nc', 'a\nb');
    const removed = rows.filter((row) => row.kind === 'del');
    expect(removed).toHaveLength(1);
    expect(removed[0]).toMatchObject({ text: 'c', oldLine: 3 });
  });

  it('emits a hunk header and keeps three lines of context', () => {
    const before = Array.from({ length: 20 }, (_, index) => `line ${index}`);
    const after = before.map((line, index) => (index === 10 ? 'line 10 changed' : line));
    const rows = computeDiffRows(before.join('\n'), after.join('\n'));
    const hunk = rows.find((row) => row.kind === 'hunk');
    expect(hunk?.text).toMatch(/^@@ -\d+,\d+ \+\d+,\d+ @@$/);
    const context = rows.filter((row) => row.kind === 'ctx');
    expect(context.length).toBeGreaterThan(0);
    expect(context.length).toBeLessThanOrEqual(8);
  });

  it('falls back to all deletions then additions past the LCS guard', () => {
    const before = Array.from({ length: 501 }, (_, index) => `old-${index}`).join('\n');
    const after = Array.from({ length: 501 }, (_, index) => `new-${index}`).join('\n');
    const rows = computeDiffRows(before, after);
    const dels = rows.filter((row) => row.kind === 'del');
    const adds = rows.filter((row) => row.kind === 'add');
    expect(dels).toHaveLength(501);
    expect(adds).toHaveLength(501);
    expect(rows[1]?.kind).toBe('del');
    expect(rows[rows.length - 1]?.kind).toBe('add');
  });
});
