import { SideView } from '@bindings/apps/desktop';

import { clubs, fullNames } from './names';

const side = (names: string[], clubList: string[]): SideView => ({
  entrant: 'e1',
  bib: 1,
  names,
  clubs: clubList,
  withdrawn: false,
});

describe('names', () => {
  it('joins full names and keeps each club once', () => {
    const s = side(['Victor Gauthier', 'David Le Michel'], ['AS Bron', 'AS Bron']);
    expect(fullNames(s)).toBe('Victor Gauthier / David Le Michel');
    expect(clubs(s)).toBe('AS Bron');
  });

  it('keeps a single name and distinct clubs', () => {
    const s = side(['Zoé'], ['A', 'B', '']);
    expect(fullNames(s)).toBe('Zoé');
    expect(clubs(s)).toBe('A / B');
  });
});
