import { ComponentFixture, TestBed } from '@angular/core/testing';
import { Router } from '@angular/router';
import { RecentTournament, TournamentView } from '@bindings/apps/desktop';

import { FakeBackend, backendFailure } from '../../testing/backend';
import { click, clickAll, render, settle, text } from '../../testing/render';
import recent from '../../testing/fixtures/recent.json';
import tournament from '../../testing/fixtures/progress-tournament.json';
import { TournamentStore } from '../core/tournament.service';
import { Shell } from './shell';

/** Renders the shell on a tournament screen. */
async function inTournament(): Promise<ComponentFixture<Shell>> {
  const f = await render(Shell);
  await TestBed.inject(Router).navigateByUrl('/tournament/desk');
  await settle(f);
  return f;
}

describe('Shell', () => {
  let go: FakeBackend;

  beforeEach(() => {
    go = new FakeBackend()
      .on('TournamentService.GetTournamentView', tournament as unknown as TournamentView)
      .on('TournamentService.ListRecentTournaments', recent as unknown as RecentTournament[])
      .on('DisplayScreenService.IsOpen', false)
      .on('DisplayScreenService.ListMonitors', []);
  });

  it('shows the tournament, its last action and undoes it', async () => {
    const f = await inTournament();
    expect(text(f)).toContain(tournament.name);
    await click(f, 'Undo');
    expect(go.called('TournamentService.Undo')).toBe(true);
  });

  it('shows a refused command until dismissed', async () => {
    const f = await inTournament();
    f.debugElement.injector.get(TournamentStore).error.set({ code: 'COURT_BUSY' });
    f.detectChanges();
    expect(text(f)).toContain('court');
    await clickAll(f, ['Undo']);
  });

  it('opens the display screen and drives the window', async () => {
    const f = await inTournament();
    go.emit('displayScreen.changed', true);
    f.detectChanges();
    await clickAll(f);
    expect(go.called('DisplayScreenService.Open') || go.called('DisplayScreenService.Close')).toBe(true);
    f.nativeElement.querySelector('header')?.dispatchEvent(new MouseEvent('dblclick', { bubbles: true }));
  });

  it('still works when the undo fails', async () => {
    go.on('TournamentService.Undo', () => {
      throw backendFailure('NOTHING_TO_UNDO');
    });
    const f = await inTournament();
    await click(f, 'Undo');
    expect(text(f)).toBeTruthy();
  });

  it('opens the guide on the website, in the UI language', async () => {
    const f = await inTournament();
    const guide = f.nativeElement.querySelector('.guide-btn') as HTMLButtonElement;
    expect(guide.getAttribute('aria-label')).toContain('Guide');
    guide.click();
  });
});
