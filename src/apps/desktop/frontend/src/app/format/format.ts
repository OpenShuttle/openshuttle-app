import { Component, computed, inject, signal, viewChild } from '@angular/core';
import { RouterLink } from '@angular/router';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { FormatView, TournamentService } from '@bindings/apps/desktop';
import { Category } from '@bindings/internal/engine';
import { StageConfig, StageKind } from '@bindings/internal/tournament';

import { liveView } from '../core/live-view';
import { AppPresets } from '../core/presets.service';
import { TournamentStore } from '../core/tournament.service';
import { FormatEditor, LAST_STAGE } from '../format-editor/format-editor';

/** Format of the open tournament, per category: the shared editor, applied through Go. */
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
  private readonly editor = viewChild(FormatEditor);

  private readonly selectedCategory = signal<Category | null>(null);
  protected readonly current = computed(() => {
    const categories = this.view().categories ?? [];
    return categories.find((c) => c.category === this.selectedCategory()) ?? categories[0] ?? null;
  });

  /** The save-as-preset form, when open. */
  protected readonly saving = signal(false);
  protected readonly presetName = signal('');
  protected readonly presetDescription = signal('');

  protected selectCategory(category: Category): void {
    this.selectedCategory.set(category);
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
