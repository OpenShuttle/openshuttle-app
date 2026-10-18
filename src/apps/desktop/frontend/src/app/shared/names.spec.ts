import { SideView } from '@bindings/apps/desktop';

import { clubs, fullNames, shortNames } from './names';

const side = (names: string[], clubList: string[]): SideView => ({
  entrant: 'e1',
  bib: 1,
  names,
  clubs: clubList,
  withdrawn: false,
});

describe('names', () => {
  it('shortens first names to initials', () => {
    const s = side(['Victor Gauthier', 'David Le Michel'], ['AS Bron', 'AS Bron']);
    expect(fullNames(s)).toBe('Victor Gauthier / David Le Michel');
    expect(shortNames(s)).toBe('V. Gauthier / D. Le Michel');
    expect(clubs(s)).toBe('AS Bron');
  });

  it('keeps a single name and distinct clubs', () => {
    const s = side(['Zoé'], ['A', 'B', '']);
    expect(shortNames(s)).toBe('Zoé');
    expect(clubs(s)).toBe('A / B');
  });
});
