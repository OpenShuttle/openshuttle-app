import { TournamentView } from '@bindings/apps/desktop';

import { FakeBackend } from '../../testing/backend';
import { render, text } from '../../testing/render';
import before from '../../testing/fixtures/before-tournament.json';
import finished from '../../testing/fixtures/finished-tournament.json';
import progress from '../../testing/fixtures/progress-tournament.json';
import { Panels } from '../core/panels.service';
import { TournamentStore } from '../core/tournament.service';
import { TournamentLayout } from './tournament-layout';

describe('TournamentLayout', () => {
  beforeEach(() => localStorage.clear());

  it.each([
    ['before the draw', before],
    ['in progress', progress],
    ['finished', finished],
  ])('lists the steps %s', async (_name, view) => {
    new FakeBackend().on('TournamentService.GetTournamentView', view as unknown as TournamentView);
    const f = await render(TournamentLayout);
    const store = f.debugElement.injector.get(TournamentStore);
    await store.refresh();
    f.detectChanges();
    expect(text(f)).toContain('Check-in');
    expect(text(f)).toContain(view.path.split('/').pop()!.slice(0, 10));
  });

  it('folds the menu into icons with instant tooltips', async () => {
    new FakeBackend().on('TournamentService.GetTournamentView', progress as unknown as TournamentView);
    const f = await render(TournamentLayout);
    (f.nativeElement.querySelector('.edge-toggle') as HTMLButtonElement).click();
    f.detectChanges();
    expect(f.debugElement.injector.get(Panels).collapsed('nav')).toBe(true);
    const item = f.nativeElement.querySelector('a.item') as HTMLElement;
    item.dispatchEvent(new MouseEvent('mouseenter'));
    f.detectChanges();
    expect(f.nativeElement.querySelector('.tip')).not.toBeNull();
    item.dispatchEvent(new MouseEvent('mouseleave'));
    f.detectChanges();
    expect(f.nativeElement.querySelector('.tip')).toBeNull();
  });
});
