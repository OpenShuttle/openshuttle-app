import { HistoryView, PhasesView, TournamentView } from '@bindings/apps/desktop';

import { FakeBackend } from '../../testing/backend';
import { click, clickAll, render, text } from '../../testing/render';
import finished from '../../testing/fixtures/finished-phases.json';
import tournament from '../../testing/fixtures/finished-tournament.json';
import history from '../../testing/fixtures/progress-history.json';
import progress from '../../testing/fixtures/progress-phases.json';
import { TournamentStore } from '../core/tournament.service';
import { Phases } from './phases';

describe('Phases', () => {
  let go: FakeBackend;
  const view = (v: unknown) => go.on('TournamentService.GetPhasesView', v as PhasesView);

  beforeEach(() => {
    localStorage.clear();
    go = new FakeBackend()
      .on('TournamentService.GetTournamentView', tournament as unknown as TournamentView)
      .on('TournamentService.GetHistoryView', history as unknown as HistoryView);
    view(finished);
  });

  it('shows the pool standings, then the bracket', async () => {
    const f = await render(Phases);
    expect(text(f)).toContain('Pool A');
    await click(f, 'Tableau principal');
    expect(text(f)).toContain('Final');
  });

  it('shows the history and undoes the latest action', async () => {
    const f = await render(Phases);
    await f.debugElement.injector.get(TournamentStore).refresh();
    f.detectChanges();
    expect(text(f)).toContain('History');
    await click(f, 'Hide the history');
    expect(text(f)).not.toContain('Undo the latest');
    await click(f, 'Show the history');
    await click(f, 'Undo the latest');
    expect(go.called('TournamentService.Undo')).toBe(true);
  });

  it('corrects scores in correction mode', async () => {
    const f = await render(Phases);
    (f.nativeElement.querySelector('.switch, [role=switch], input[type=checkbox]') as HTMLElement)?.click();
    await clickAll(f, ['Undo']);
    await click(f, 'Tableau principal');
    await clickAll(f, ['Undo']);
  });

  it('runs every button of a tournament in progress', async () => {
    view(progress);
    const f = await render(Phases);
    await clickAll(f, ['Undo']);
    await click(f, "Women's doubles");
    await clickAll(f, ['Undo']);
  });
});
