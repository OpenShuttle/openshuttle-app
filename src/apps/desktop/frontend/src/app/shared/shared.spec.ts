import { Component, signal } from '@angular/core';
import { Presets, Settings } from '@bindings/apps/desktop';
import { Category } from '@bindings/internal/engine';

import { FakeBackend, backendFailure } from '../../testing/backend';
import { choose, click, find, render, text, type } from '../../testing/render';
import presets from '../../testing/fixtures/presets.json';
import settings from '../../testing/fixtures/settings.json';
import { CategoryBadge } from './category-badge';
import { ClubSelect, clubKey, findClub } from './club-select';
import { Glide } from './glide';
import { PanelToggle } from './panel-strip';

describe('shared components', () => {
  beforeEach(() => {
    localStorage.clear();
    new FakeBackend().on('PresetService.GetPresets', presets as unknown as Presets).on('SettingsService.GetSettings', settings as unknown as Settings);
  });

  it('CategoryBadge shows the short code in the UI language', async () => {
    let f = await render(CategoryBadge, { inputs: { category: 'MD' as Category, slot: 1 } });
    expect(text(f)).toBe('MD');
    f = await render(CategoryBadge, { lang: 'fr', inputs: { category: 'MD' as Category, slot: 2, ring: true } });
    expect(text(f)).toBe('DH');
  });

  it('PanelToggle folds a panel into a strip and back', async () => {
    const f = await render(PanelToggle, { inputs: { key: 'info.file', title: 'File' } });
    (find(f, 'button') as HTMLButtonElement).click();
    f.detectChanges();
    expect(text(f)).toContain('File');
    (find(f, 'button') as HTMLButtonElement).click();
    f.detectChanges();
    expect(f.nativeElement.querySelector('.strip')).toBeNull();
  });

  it('ClubSelect picks a known club or adds a new one', async () => {
    const go = new FakeBackend().on('PresetService.GetPresets', presets as unknown as Presets).on('SettingsService.GetSettings', settings as unknown as Settings);
    const changes: string[] = [];
    @Component({
      imports: [ClubSelect],
      template: `<app-club-select [value]="club()" (changed)="changes.push($event); club.set($event)" />`,
    })
    class Host {
      readonly club = signal('');
      readonly changes = changes;
    }
    const f = await render(Host);
    const select = () => find(f, 'select') as HTMLSelectElement;
    // A listed club shows its acronym.
    expect([...select().options].map((o) => o.textContent!.trim())).toContain('BC Rive Droite (BCRD)');
    await choose(f, select(), 'BC Rive Droite');
    expect(changes).toEqual(['BC Rive Droite']);

    // Typed by its acronym, a listed club is picked, not added.
    await choose(f, select(), '__new');
    const [name] = [...f.nativeElement.querySelectorAll('input')] as HTMLInputElement[];
    await type(f, name, ' vdm ');
    await click(f, 'Add');
    expect(changes).toEqual(['BC Rive Droite', 'Volant des Monts']);
    expect(go.called('PresetService.SaveClub')).toBe(false);

    // A new club with its acronym is added to the list.
    await choose(f, select(), '__new');
    const [name2, acronym2] = [...f.nativeElement.querySelectorAll('input')] as HTMLInputElement[];
    await type(f, name2, 'Rillieux');
    await type(f, acronym2, 'AUVR');
    await click(f, 'Add');
    expect(go.argsOf('PresetService.SaveClub')).toEqual([['', { name: 'Rillieux', acronym: 'AUVR' }]]);
    expect(changes.at(-1)).toBe('Rillieux');

    // A refused club says why and stays open.
    go.on('PresetService.SaveClub', () => {
      throw backendFailure('CLUB_EXISTS');
    });
    await choose(f, select(), '__new');
    const [name3, acronym3] = [...f.nativeElement.querySelectorAll('input')] as HTMLInputElement[];
    await type(f, name3, 'Autre');
    await type(f, acronym3, 'BCRD');
    await click(f, 'Add');
    expect(text(f)).toContain('Another club already has this name or acronym.');
  });

  it('clubKey and findClub match clubs by name or acronym, as Go does', () => {
    expect(clubKey('Rillieux-la-Pape')).toBe(clubKey('rillieux la pape'));
    expect(clubKey('A.U.V.R.')).toBe('auvr');
    expect(clubKey('Évian')).toBe('evian');
    const list = [{ name: 'Rillieux', acronym: 'AUVR' }, { name: 'AUVR Bis', acronym: '' }];
    expect(findClub(list, 'auvr')?.name).toBe('Rillieux');
    expect(findClub(list, 'RILLIEUX')?.name).toBe('Rillieux');
    expect(findClub(list, 'PLVPB')).toBeUndefined();
    expect(findClub(list, '')).toBeUndefined();
  });

  it('Glide animates what moved since the last render', () => {
    const host = document.createElement('div');
    const a = document.createElement('div');
    a.dataset['flip'] = 'a';
    host.appendChild(a);
    document.body.appendChild(host);
    const animate = vi.fn(() => ({}) as Animation);
    Object.assign(a, { animate, getAnimations: () => [] });
    const glide = new Glide(host);
    glide.run();
    Object.defineProperty(a, 'offsetTop', { value: 40, configurable: true });
    glide.run();
    expect(animate).toHaveBeenCalledTimes(1);
    vi.stubGlobal('DOMMatrix', class {
      e = 5;
      f = 7;
    });
    const cancel = vi.fn();
    Object.assign(a, { getAnimations: () => [{ cancel }] });
    Object.defineProperty(a, 'offsetTop', { value: 80, configurable: true });
    glide.run();
    expect(cancel).toHaveBeenCalled();
    vi.unstubAllGlobals();
    host.remove();
  });
});
