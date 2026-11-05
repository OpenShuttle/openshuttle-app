import { FormatView, Presets, Settings, StageView } from '@bindings/apps/desktop';
import { StageConfig } from '@bindings/internal/tournament';

import { FakeBackend } from '../../testing/backend';
import { choose, click, clickAll, render, text } from '../../testing/render';
import format from '../../testing/fixtures/before-format.json';
import presets from '../../testing/fixtures/presets.json';
import settings from '../../testing/fixtures/settings.json';
import { FormatEditor, canAddStage } from './format-editor';

const stages = (format as unknown as FormatView).categories![0].previews![0].stages as StageView[];

describe('FormatEditor', () => {
  beforeEach(() => {
    new FakeBackend()
      .on('PresetService.GetPresets', presets as unknown as Presets)
      .on('SettingsService.GetSettings', settings as unknown as Settings)
      .on('PresetService.EstimateMinutes', 22);
  });

  async function editor(readonly = false, of: StageView[] = stages) {
    const f = await render(FormatEditor, { inputs: { stages: of, entrants: 12, courts: 4, ranked: 12, readonly } });
    const changes: StageConfig[][] = [];
    f.componentInstance.changed.subscribe((c) => changes.push(c));
    return { f, changes };
  }

  it('only allows a new stage after a pools stage', () => {
    expect(canAddStage(stages)).toBe(true);
    expect(canAddStage(stages.filter((s) => s.kind === 'bracket'))).toBe(false);
  });

  it('edits every setting of every stage', async () => {
    const { f, changes } = await editor();
    for (const node of [...f.nativeElement.querySelectorAll('.node:not(.output)')] as HTMLElement[]) {
      node.click();
      f.detectChanges();
      await clickAll(f, ['Remove']);
      for (const select of [...f.nativeElement.querySelectorAll('.panel select, aside select')] as HTMLSelectElement[]) {
        const last = select.options[select.options.length - 1];
        if (last?.value) {
          await choose(f, select, last.value);
        }
      }
    }
    expect(changes.length).toBeGreaterThan(10);
    expect(text(f)).toContain('Tableau principal');
  });

  it('sets a stage acronym, the kind default as placeholder', async () => {
    const { f, changes } = await editor();
    const input = f.nativeElement.querySelector('[data-acronym]') as HTMLInputElement;
    expect(input.placeholder).toBe('P');
    input.value = 'Q';
    input.dispatchEvent(new Event('change'));
    expect(changes.at(-1)?.[0].acronym).toBe('Q');
  });

  it('sets the pause before a later stage, by quarter hours or an hour at once', async () => {
    const { f, changes } = await editor();
    // The entry stage has none: nothing feeds it.
    expect(f.nativeElement.querySelector('[data-pause]')).toBeNull();
    const later = [...f.nativeElement.querySelectorAll('.node:not(.output)')][1] as HTMLElement;
    later.click();
    f.detectChanges();
    const pause = f.nativeElement.querySelector('[data-pause]') as HTMLElement;
    expect(pause.textContent).toContain('None');
    (pause.querySelector('[aria-label="Longer pause"]') as HTMLButtonElement).click();
    expect(changes.at(-1)?.[1].pause).toBe(15);
    await click(f, '1 h', '[data-pause] button');
    expect(changes.at(-1)?.[1].pause).toBe(60);
    // The other stages keep their settings.
    expect(changes.at(-1)?.[0]).toEqual(expect.objectContaining({ id: stages[0].id, acronym: stages[0].acronym }));
  });

  it('shows the pause in hours and minutes', async () => {
    const paused = stages.map((s, i) => (i === 1 ? { ...s, pause: 90 } : s));
    const { f, changes } = await editor(false, paused);
    ([...f.nativeElement.querySelectorAll('.node:not(.output)')][1] as HTMLElement).click();
    f.detectChanges();
    expect(f.nativeElement.querySelector('[data-pause]').textContent).toContain('1 h 30');
    (f.nativeElement.querySelector('[aria-label="Shorter pause"]') as HTMLButtonElement).click();
    expect(changes.at(-1)?.[1].pause).toBe(75);
  });

  it('warns that reshaping a filled later stage empties it', async () => {
    const filled = stages.map((s) => (s.entry ? s : { ...s, filled: true, confirmed: true }));
    const { f, changes } = await editor(false, filled);
    const later = [...f.nativeElement.querySelectorAll('.node:not(.output)')][1] as HTMLElement;
    later.click();
    f.detectChanges();
    expect(text(f)).toContain('empties it');
    // Its kind can still change: the desk fills it again.
    const kinds = [...f.nativeElement.querySelectorAll('.panel-body .seg')][0].querySelectorAll('button:enabled');
    expect(kinds.length).toBe(2);
    await clickAll(f, ['Pools']);
    expect(changes.length).toBeGreaterThan(0);
  });

  it('shows a built-in format without editing it', async () => {
    const { f } = await editor(true);
    // The panel's fieldset is disabled: none of its controls can be used.
    expect((f.nativeElement.querySelector('fieldset.panel-body') as HTMLFieldSetElement).disabled).toBe(true);
    expect(f.nativeElement.querySelectorAll('fieldset.panel-body button:enabled').length).toBe(0);
  });
});
