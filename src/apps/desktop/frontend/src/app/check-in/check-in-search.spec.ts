import { CheckInRow } from '@bindings/apps/desktop';
import { Category } from '@bindings/internal/engine';

import { fold, matches, ownText } from './check-in-search';

const row = (firstName: string, lastName: string, club = '', bib = 0): CheckInRow => ({
  id: 'p1',
  firstName,
  lastName,
  club,
  checkedIn: false,
  checkedInAt: null,
  duplicate: false,
  entrantId: 'e1',
  category: Category.MensDoubles,
  slot: 1,
  bib,
  partner: null,
});

describe('check-in search', () => {
  it('ignores case and accents', () => {
    expect(fold('Écuyer Inès')).toBe('ecuyer ines');
    expect(matches(ownText(row('Hugo', 'Écuyer')), ['ecu'])).toBe(true);
  });

  it('needs every word, in any field', () => {
    expect(matches(ownText(row('Paul', 'Zola', 'BC Lyon')), ['zola', 'p'])).toBe(true);
    expect(matches(ownText(row('Paul', 'Zola', 'BC Lyon')), ['zola', 'lyon'])).toBe(true);
    expect(matches(ownText(row('Hugo', 'Ecuyer')), ['zola', 'p'])).toBe(false);
  });

  it('finds a bib', () => {
    expect(matches(ownText(row('Paul', 'Zola', '', 12)), ['12'])).toBe(true);
    expect(matches(ownText(row('Paul', 'Zola')), ['0'])).toBe(false);
  });
});
