import { RecentTournament, TournamentView } from '@bindings/apps/desktop';

import { FakeBackend } from '../../testing/backend';
import { click, clickAll, find, render, text, type } from '../../testing/render';
import recent from '../../testing/fixtures/recent.json';
import tournament from '../../testing/fixtures/finished-tournament.json';
import { Home } from './home';

describe('Home', () => {
  let go: FakeBackend;

  beforeEach(() => {
    go = new FakeBackend()
      .on('TournamentService.GetTournamentView', { ...tournament, open: false } as unknown as TournamentView)
      .on('TournamentService.ListRecentTournaments', recent as unknown as RecentTournament[]);
  });

  it('lists the recent tournaments and opens one', async () => {
    const f = await render(Home);
    expect(text(f)).toContain(recent[0].name);
    await click(f, 'Open');
  });

  it('creates a tournament', async () => {
    go.on('TournamentService.CreateTournament', '/t.openshuttle');
    const f = await render(Home);
    await click(f, 'New tournament');
    await type(f, find(f, 'form input') as HTMLInputElement, 'Plume');
    (find(f, 'form') as HTMLFormElement).dispatchEvent(new Event('submit'));
    await new Promise((r) => setTimeout(r));
    expect(go.argsOf('TournamentService.CreateTournament')).toEqual([['Plume']]);
  });

  it('runs every button', async () => {
    go.on('TournamentService.DuplicateTournament', '');
    await clickAll(await render(Home));
  });
});
