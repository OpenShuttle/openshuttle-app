import { Component, computed, inject, signal, viewChild } from '@angular/core';
import { RouterLink } from '@angular/router';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { FormatCategory, FormatPreviewView, FormatView, PreviewBase, TournamentService } from '@bindings/apps/desktop';
import { Category } from '@bindings/internal/engine';
import { StageConfig, StageKind } from '@bindings/internal/tournament';

import { CategoryShown } from '../core/category-shown.service';
import { liveView } from '../core/live-view';
import { AppPresets } from '../core/presets.service';
import { TournamentStore } from '../core/tournament.service';
import { FormatEditor, LAST_STAGE, canAddStage } from '../format-editor/format-editor';

const BASE_KEY = 'openshuttle.formatPreviewBase';

function loadBase(): PreviewBase {
  try {
    return (localStorage.getItem(BASE_KEY) as PreviewBase | null) ?? PreviewBase.PreviewPaired;
  } catch {
    return PreviewBase.PreviewPaired;
  }
}

/**
 * Format of the open tournament, per category: the shared editor, applied
 * through Go. The preview counts complete pairs, every entrant or the present
 * ones: Go previews each base, the choice is a convenience of this PC.
 */
@Component({
  selector: 'app-format',
  imports: [TranslocoDirective, RouterLink, FormatEditor],
  templateUrl: './format.html',
  styleUrl: './format.scss',
})
export class Format {
  protected readonly tournament = inject(TournamentStore);
  protected readonly presets = inject(AppPresets);
  private readonly transloco = inject(TranslocoService);
  protected readonly view = liveView<FormatView>(() => TournamentService.GetFormatView(), { categories: [] });
  protected readonly stageKinds = StageKind;
  protected readonly canAdd = computed(() => canAddStage(this.preview()?.stages ?? []));
  private readonly editor = viewChild(FormatEditor);

  private readonly shown = inject(CategoryShown);
  protected readonly current = computed(() => this.shown.pick(this.view().categories));

  /** The base of the preview; singles have no paired base and show every entrant. */
  protected readonly base = signal<PreviewBase>(loadBase());
  protected readonly preview = computed(() => {
    const fc = this.current();
    return fc ? this.previewOf(fc) : null;
  });

  protected previewOf(fc: FormatCategory): FormatPreviewView | null {
    const previews = fc.previews ?? [];
    return (
      previews.find((p) => p.base === this.base()) ??
      previews.find((p) => p.base === PreviewBase.PreviewRegistered) ??
      previews[0] ??
      null
    );
  }

  protected selectBase(base: PreviewBase): void {
    this.base.set(base);
    try {
      localStorage.setItem(BASE_KEY, base);
    } catch {
      // Not saved: the choice still applies for this session.
    }
  }

  /** The save-as-preset form, when open. */
  protected readonly saving = signal(false);
  protected readonly presetName = signal('');
  protected readonly presetDescription = signal('');

  protected selectCategory(category: Category): void {
    this.shown.show(category);
    this.editor()?.selectedID.set('');
  }

  protected change(stages: StageConfig[]): void {
    const fc = this.current();
    if (fc) {
      void this.tournament.setFormat(fc.category, stages);
    }
  }

  protected async add(kind: StageKind): Promise<void> {
    const fc = this.current();
    if (fc && (await this.tournament.addStage(fc.category, kind))) {
      this.editor()?.selectedID.set(LAST_STAGE);
    }
  }

  protected remove(id: string): void {
    const fc = this.current();
    if (fc) {
      void this.tournament.removeStage(fc.category, id);
      this.editor()?.selectedID.set('');
    }
  }

  protected applyFormatPreset(event: Event): void {
    const fc = this.current();
    const select = event.target as HTMLSelectElement;
    const id = select.value;
    select.value = '';
    if (fc && id) {
      void this.tournament.applyFormatPreset(fc.category, id);
      this.editor()?.selectedID.set('');
    }
  }

  protected formatPresetLabel(p: { key: string; name: string }): string {
    return p.name || this.transloco.translate('presets.format.' + p.key + '.name');
  }

  protected openSave(): void {
    this.presetName.set('');
    this.presetDescription.set('');
    this.saving.set(true);
  }

  protected async savePreset(): Promise<void> {
    const fc = this.current();
    if (fc && (await this.tournament.saveFormatPreset(fc.category, this.presetName(), this.presetDescription()))) {
      this.saving.set(false);
    }
  }

  protected value(event: Event): string {
    return (event.target as HTMLInputElement).value;
  }
}
