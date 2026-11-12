import { describe, expect, it } from 'vitest';
import { catalogFilter } from './catalog';

describe('catalogFilter', () => {
  it('joins official and secondary filters', () => {
    expect(catalogFilter('official', 'stdio')).toBe('official,stdio');
  });

  it('appends extra platform tokens', () => {
    expect(catalogFilter('all', '', 'darwin')).toBe('darwin');
    expect(catalogFilter('community', '', 'linux')).toBe('community,linux');
  });
});
