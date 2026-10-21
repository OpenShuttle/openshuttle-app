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

  it('builds the sample tournament and opens its control desk', async () => {
    go.on('TournamentService.CreateSampleTournament', '/home/julie/Tournoi d\'exemple.openshuttle');
    const f = await render(Home);
    await click(f, 'Try with a sample tournament');
    expect(go.argsOf('TournamentService.CreateSampleTournament')).toEqual([['Sample tournament']]);
  });

  it('offers the sample tournament and the guide on first launch', async () => {
    go.on('TournamentService.ListRecentTournaments', []);
    const f = await render(Home, { lang: 'fr' });
    expect(text(f)).toContain("Essayer avec un tournoi d'exemple");
    await click(f, 'Lire le guide');
    await click(f, "Essayer avec un tournoi d'exemple");
    expect(go.argsOf('TournamentService.CreateSampleTournament')).toEqual([["Tournoi d'exemple"]]);
  });
});
