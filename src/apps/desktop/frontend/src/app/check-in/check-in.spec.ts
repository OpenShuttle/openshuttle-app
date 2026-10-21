import { CheckInView, TournamentView } from '@bindings/apps/desktop';

import { FakeBackend } from '../../testing/backend';
import { click, clickAll, find, render, text, type } from '../../testing/render';
import checkIn from '../../testing/fixtures/before-checkin.json';
import tournament from '../../testing/fixtures/before-tournament.json';
import { CheckIn } from './check-in';

describe('CheckIn', () => {
  let go: FakeBackend;

  beforeEach(() => {
    go = new FakeBackend()
      .on('TournamentService.GetTournamentView', tournament as unknown as TournamentView)
      .on('TournamentService.GetCheckInView', checkIn as unknown as CheckInView);
  });

  it('counts the players present', async () => {
    const f = await render(CheckIn);
    expect(text(f)).toMatch(new RegExp(`${checkIn.present}\\s*/ ${checkIn.players}`));
  });

  it('checks in a pair, a partner, or one player', async () => {
    const f = await render(CheckIn);
    await click(f, 'Check in the pair');
    expect(go.called('TournamentService.CheckInEntrant')).toBe(true);
    await click(f, 'Check in the partner');
    const player = f.nativeElement.querySelector('button.player') as HTMLButtonElement;
    player.click();
    expect(go.called('TournamentService.CheckInPlayer') || go.called('TournamentService.CancelCheckIn')).toBe(true);
  });

  it('searches and checks in the only match with Enter', async () => {
    const f = await render(CheckIn);
    const absent = checkIn.teams.flatMap((t) => t.players ?? []).find((p) => !p.checkedIn)!;
    const search = find(f, 'input[type=search]') as HTMLInputElement;
    await type(f, search, absent.name);
    expect(text(f)).toContain(absent.name);
    search.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter' }));
    await new Promise((r) => setTimeout(r));
    expect(go.argsOf('TournamentService.CheckInPlayer')).toEqual([[absent.id]]);
    search.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
  });

  it('runs every button', async () => {
    await clickAll(await render(CheckIn));
  });
});
