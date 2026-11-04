import { DrawView, TournamentView } from '@bindings/apps/desktop';

import { FakeBackend } from '../../testing/backend';
import { click, clickAll, drag, render, text } from '../../testing/render';
import draw from '../../testing/fixtures/before-draw.json';
import tournament from '../../testing/fixtures/before-tournament.json';
import { Draw } from './draw';

describe('Draw', () => {
  let go: FakeBackend;

  beforeEach(() => {
    go = new FakeBackend()
      .on('TournamentService.GetTournamentView', tournament as unknown as TournamentView)
      .on('TournamentService.GetDrawView', draw as unknown as DrawView);
  });

  it('shows the drawn pools and the seeds', async () => {
    const f = await render(Draw);
    expect(text(f)).toContain(draw.categories[0].pools![0].entrants![0].players![0].name);
    expect(text(f)).toContain(draw.categories[0].ranking![0].players![0].name);
  });

  it('shows clubs as colored badges, club mates of a pool highlighted', async () => {
    const view = structuredClone(draw);
    const [first, second] = view.categories[0].pools![0].entrants!;
    first.clubs = [{ name: 'BC Lyon', color: 3, count: 2 }];
    second.clubs = [
      { name: 'BC Lyon', color: 3, count: 2 },
      { name: 'AS Caen', color: 1, count: 1 },
    ];
    go.on('TournamentService.GetDrawView', view);
    const f = await render(Draw);
    const badges = [...f.nativeElement.querySelectorAll('[data-pool="MD:0"] .club-badge')] as HTMLElement[];
    expect(badges.map((b) => b.textContent!.replace(/\s+/g, ' ').trim())).toEqual(
      expect.arrayContaining(['BC Lyon×2', 'AS Caen']),
    );
    const lyon = badges.find((b) => b.textContent!.includes('Lyon'))!;
    expect(lyon.classList).toContain('shared');
    expect(lyon.style.getPropertyValue('--club')).toBe('var(--color-club-3)');
    expect(lyon.title).toBe('BC Lyon: 2 entrants of this club together');
    const caen = badges.find((b) => b.textContent!.includes('Caen'))!;
    expect(caen.classList).not.toContain('shared');
    expect(caen.title).toBe('AS Caen');
  });

  it('colors the rank chips by ranking level', async () => {
    const view = structuredClone(draw);
    const [first, second] = view.categories[0].pools![0].entrants!;
    Object.assign(first, { seed: 1, tier: 1 });
    Object.assign(second, { seed: 12, tier: 10 });
    go.on('TournamentService.GetDrawView', view);
    const f = await render(Draw, { lang: 'fr' });
    const chips = [...f.nativeElement.querySelectorAll('[data-pool="MD:0"] .seed')] as HTMLElement[];
    expect(chips.map((c) => c.textContent!.trim())).toEqual(['C1', 'C12']);
    expect(chips[0].style.getPropertyValue('--tier')).toBe('var(--color-tier-1)');
    expect(chips[1].style.getPropertyValue('--tier')).toBe('var(--color-tier-2)'); // 8 colors, then again
    expect(chips[1].title).toBe('Classé 12 · niveau 10');
    expect(text(f)).not.toContain('Même club');
  });

  it('shows no club badge for an unclubbed entrant', async () => {
    const view = structuredClone(draw);
    const pool = view.categories[0].pools![0];
    pool.entrants!.forEach((e) => (e.clubs = []));
    go.on('TournamentService.GetDrawView', view);
    const f = await render(Draw);
    expect(f.nativeElement.querySelectorAll('[data-pool="MD:0"] .club-badge').length).toBe(0);
    expect(f.nativeElement.querySelectorAll('.seed-row .club-badge').length).toBeGreaterThan(0);
  });

  it('warns about the players waiting for a partner', async () => {
    const f = await render(Draw);
    expect(text(f)).toContain('1 player waits for a partner and stays out of the draw. Pair them in Entrants');
  });

  it('draws again, confirms and draws the other category', async () => {
    const f = await render(Draw);
    await click(f, 'Draw again');
    expect(go.argsOf('TournamentService.DrawEntry')).toEqual([['MD']]);
    await click(f, 'Confirm the draw');
    expect(go.argsOf('TournamentService.ConfirmDraw')).toEqual([['MD']]);
  });

  it('reopens a confirmed draw', async () => {
    const confirmed = structuredClone(draw);
    confirmed.categories[0].status = 'confirmed';
    go.on('TournamentService.GetDrawView', confirmed);
    const f = await render(Draw);
    await click(f, 'Reopen');
    expect(go.argsOf('TournamentService.ReopenDraw')).toEqual([['MD']]);
  });

  it('runs every button', async () => {
    const f = await render(Draw);
    await clickAll(f);
    expect(go.called('TournamentService.SetSeeds')).toBe(true);
  });

  it('moves an entrant to another pool by drag and drop', async () => {
    const f = await render(Draw);
    const card = f.nativeElement.querySelector('[data-pool="MD:0"] .entrant') as HTMLElement;
    const pool = f.nativeElement.querySelector('[data-pool="MD:2"]') as HTMLElement;
    await drag(f, card, pool);
    expect(go.argsOf('TournamentService.MoveEntrant')).toEqual([[draw.categories[0].pools![0].entrants![0].id, 2]]);
  });

  it('ranks the seeds by drag and drop', async () => {
    const f = await render(Draw);
    const rows = [...f.nativeElement.querySelectorAll('.seed-row')] as HTMLElement[];
    expect(rows.length).toBeGreaterThan(2);
    const unseeded = rows.find((r) => !r.dataset['rank'])!; // a men's pair not seeded
    await drag(f, unseeded, rows[0]); // the lower half of the top seed's row: just after it
    const md = draw.categories[0];
    const [top, ...others] = md.ranking!.map((e) => e.id);
    expect(go.argsOf('TournamentService.SetSeeds')[0]).toEqual(['MD', [top, md.unseeded![0].id, ...others]]);
    await drag(f, rows[0], f.nativeElement.querySelector('[data-unseeded="MD"]'));
    expect(go.argsOf('TournamentService.SetSeeds')[1]).toEqual(['MD', md.ranking!.slice(1).map((e) => e.id)]);
  });
});
