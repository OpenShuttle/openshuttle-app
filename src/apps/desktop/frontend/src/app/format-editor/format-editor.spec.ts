import { FormatView, Presets, Settings, StageView } from '@bindings/apps/desktop';
import { StageConfig } from '@bindings/internal/tournament';

import { FakeBackend } from '../../testing/backend';
import { choose, clickAll, render, text } from '../../testing/render';
import format from '../../testing/fixtures/before-format.json';
import presets from '../../testing/fixtures/presets.json';
import settings from '../../testing/fixtures/settings.json';
import { FormatEditor, canAddStage } from './format-editor';

const stages = (format as unknown as FormatView).categories![0].stages as StageView[];

describe('FormatEditor', () => {
  beforeEach(() => {
    new FakeBackend()
      .on('PresetService.GetPresets', presets as unknown as Presets)
      .on('SettingsService.GetSettings', settings as unknown as Settings)
      .on('PresetService.EstimateMinutes', 22);
  });

  async function editor(readonly = false) {
    const f = await render(FormatEditor, { inputs: { stages, entrants: 12, courts: 4, ranked: 12, readonly } });
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

  it('shows a built-in format without editing it', async () => {
    const { f } = await editor(true);
    // The panel's fieldset is disabled: none of its controls can be used.
    expect((f.nativeElement.querySelector('fieldset.panel-body') as HTMLFieldSetElement).disabled).toBe(true);
    expect(f.nativeElement.querySelectorAll('fieldset.panel-body button:enabled').length).toBe(0);
  });
});
