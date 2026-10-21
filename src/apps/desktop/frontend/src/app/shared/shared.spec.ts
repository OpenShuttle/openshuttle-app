import { Component, signal } from '@angular/core';
import { Presets, Settings } from '@bindings/apps/desktop';
import { Category } from '@bindings/internal/engine';

import { FakeBackend } from '../../testing/backend';
import { choose, find, render, text, type } from '../../testing/render';
import presets from '../../testing/fixtures/presets.json';
import settings from '../../testing/fixtures/settings.json';
import { CategoryBadge } from './category-badge';
import { ClubSelect } from './club-select';
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
    const select = find(f, 'select') as HTMLSelectElement;
    await choose(f, select, presets.clubs![0]);
    expect(changes).toEqual([presets.clubs![0]]);
    const newOption = [...select.options].find((o) => o.value && !presets.clubs!.includes(o.value) && o.value !== '');
    if (newOption) {
      await choose(f, select, newOption.value);
      const input = f.nativeElement.querySelector('input') as HTMLInputElement | null;
      if (input) {
        await type(f, input, 'Nouveau club');
        input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter' }));
        input.dispatchEvent(new Event('blur'));
      }
    }
    expect(changes.length).toBeGreaterThan(0);
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
