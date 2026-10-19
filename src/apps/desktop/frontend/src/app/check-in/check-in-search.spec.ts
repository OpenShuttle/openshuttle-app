import { CheckInTeam, PlayerView } from '@bindings/apps/desktop';
import { Category } from '@bindings/internal/engine';

import { fold, matches, ownText, teamText } from './check-in-search';

const player = (firstName: string, lastName: string, club = ''): PlayerView => ({
  id: 'p1',
  name: `${firstName} ${lastName}`,
  club,
  checkedIn: false,
  checkedInAt: null,
  duplicate: false,
});

const team = (players: PlayerView[], bib = 0): CheckInTeam => ({
  entrantId: 'e1',
  category: Category.MensDoubles,
  slot: 1,
  bib,
  players,
  complete: false,
});

const own = (firstName: string, lastName: string, club = '', bib = 0) => {
  const p = player(firstName, lastName, club);
  return ownText(p, team([p], bib));
};

describe('check-in search', () => {
  it('ignores case and accents', () => {
    expect(fold('Écuyer Inès')).toBe('ecuyer ines');
    expect(matches(own('Hugo', 'Écuyer'), ['ecu'])).toBe(true);
  });

  it('needs every word, in any field', () => {
    expect(matches(own('Paul', 'Zola', 'BC Lyon'), ['zola', 'p'])).toBe(true);
    expect(matches(own('Paul', 'Zola', 'BC Lyon'), ['zola', 'lyon'])).toBe(true);
    expect(matches(own('Hugo', 'Ecuyer'), ['zola', 'p'])).toBe(false);
  });

  it('finds a bib', () => {
    expect(matches(own('Paul', 'Zola', '', 12), ['12'])).toBe(true);
    expect(matches(own('Paul', 'Zola'), ['0'])).toBe(false);
  });

  it('finds a team by either player', () => {
    const t = team([player('Paul', 'Zola', 'BC Lyon'), player('Hugo', 'Ecuyer', 'AS Bron')], 7);
    expect(matches(teamText(t), ['hugo'])).toBe(true);
    expect(matches(teamText(t), ['zola', 'bron'])).toBe(true);
    expect(matches(teamText(t), ['7'])).toBe(true);
    expect(matches(teamText(t), ['lea'])).toBe(false);
  });
});
