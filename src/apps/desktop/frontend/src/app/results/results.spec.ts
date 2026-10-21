import { ResultsView, TournamentView } from '@bindings/apps/desktop';

import { FakeBackend } from '../../testing/backend';
import { click, clickAll, render, text } from '../../testing/render';
import before from '../../testing/fixtures/before-results.json';
import results from '../../testing/fixtures/finished-results.json';
import tournament from '../../testing/fixtures/finished-tournament.json';
import progress from '../../testing/fixtures/progress-results.json';
import { Results } from './results';

describe('Results', () => {
  let go: FakeBackend;
  const view = (v: unknown) => go.on('TournamentService.GetResultsView', v as ResultsView);

  beforeEach(() => {
    go = new FakeBackend().on('TournamentService.GetTournamentView', tournament as unknown as TournamentView);
    view(results);
  });

  it('ranks every entrant', async () => {
    const f = await render(Results);
    expect(text(f)).toContain(results.categories[0].rows![0].entrant.names![0]);
  });

  it('exports the standings as CSV and archives with a second click', async () => {
    go.on('TournamentService.ExportCSV', '/home/julie/classement.csv');
    const f = await render(Results);
    await click(f, 'Export the standings');
    const [, csv] = go.argsOf('TournamentService.ExportCSV')[0] as [string, string];
    expect(csv).toContain(results.categories[0].rows![0].entrant.names![0]);
    expect(text(f)).toContain('/home/julie/classement.csv');
    await click(f, 'Close and archive');
    expect(go.called('TournamentService.ArchiveTournament')).toBe(false);
    await click(f, 'Confirm archive');
    expect(go.called('TournamentService.ArchiveTournament')).toBe(true);
  });

  it('previews the standings while playing, nothing before', async () => {
    view(progress);
    expect(text(await render(Results))).toContain('preview');
    view(before);
    await clickAll(await render(Results));
  });
});
