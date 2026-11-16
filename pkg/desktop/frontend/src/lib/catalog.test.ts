import { describe, expect, it } from 'vitest';
import { catalogFilter, catalogVersionPair, normalizeVersion } from './catalog';
import type { CatalogItem } from '../types';

describe('catalogFilter', () => {
  it('joins official and secondary filters', () => {
    expect(catalogFilter('official', 'stdio')).toBe('official,stdio');
  });

  it('appends extra platform tokens', () => {
    expect(catalogFilter('all', '', 'darwin')).toBe('darwin');
    expect(catalogFilter('community', '', 'linux')).toBe('community,linux');
  });
});

describe('catalogVersionPair', () => {
  it('normalizes a leading v on both sides', () => {
    expect(normalizeVersion('v1.2.0')).toBe('1.2.0');
    expect(
      catalogVersionPair({ installedVersion: 'v1.2.0', version: '1.3.0' } as CatalogItem),
    ).toEqual({ from: '1.2.0', to: '1.3.0' });
  });

  it('returns null when either side is missing', () => {
    expect(catalogVersionPair({ installedVersion: '1.2.0' } as CatalogItem)).toBeNull();
    expect(catalogVersionPair({ version: '1.3.0' } as CatalogItem)).toBeNull();
  });
});
