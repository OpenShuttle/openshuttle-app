import { Component, inject } from '@angular/core';
import { TranslocoDirective } from '@jsverse/transloco';
import { HistoryView, TournamentService } from '@bindings/apps/desktop';

import { ActionLabels } from '../core/action-label';
import { LanguageService } from '../core/language.service';
import { liveView } from '../core/live-view';
import { TournamentStore } from '../core/tournament.service';

/** Column listing every action, newest first; undone ones struck out. */
@Component({
  selector: 'app-history-panel',
  imports: [TranslocoDirective],
  template: `
    <aside *transloco="let t" class="history">
      <header>
        <h2>{{ t('historyPanel.title') }}</h2>
        <button type="button" class="btn btn-sm undo" [disabled]="!tournament.view().canUndo" (click)="tournament.undo()">
          {{ t('historyPanel.undoLast') }}
        </button>
      </header>
      <ol>
        @for (e of view().entries; track e.seq) {
          <li [class.undone]="e.undone">
            <span class="time">{{ time(e.at) }}</span>
            <span class="text">
              <span class="label">{{ labels.label(e) }}</span>
              @if (e.operator) {
                <span class="operator">{{ e.operator }}</span>
              }
            </span>
          </li>
        }
      </ol>
    </aside>
  `,
  styles: `
    :host {
      display: block;
      width: 320px;
      flex: none;
      min-height: 0;
    }
    .history {
      height: 100%;
      display: flex;
      flex-direction: column;
      background: var(--color-surface);
      border-left: 1px solid var(--color-border);
    }
    header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 10px;
      padding: 18px 18px 12px;
      border-bottom: 1px solid #ecedf0;
    }
    h2 {
      margin: 0;
      font-size: 18px;
      font-weight: 800;
    }
    .undo {
      height: 34px;
      font-size: 13px;
    }
    ol {
      flex: 1;
      list-style: none;
      margin: 0;
      padding: 0;
      overflow: auto;
    }
    li {
      display: grid;
      grid-template-columns: 48px minmax(0, 1fr);
      gap: 10px;
      padding: 10px 18px;
      border-bottom: 1px solid #ecedf0;
    }
    .time {
      font: 600 13px var(--font-mono);
      color: var(--color-text-muted);
    }
    .text {
      display: flex;
      flex-direction: column;
      gap: 2px;
    }
    .label {
      font-size: 14px;
    }
    .operator {
      font-size: 12px;
      color: var(--color-text-muted);
    }
    .undone .label {
      text-decoration: line-through;
      color: var(--color-text-muted);
    }
  `,
})
export class HistoryPanel {
  protected readonly tournament = inject(TournamentStore);
  protected readonly labels = inject(ActionLabels);
  private readonly language = inject(LanguageService);
  protected readonly view = liveView<HistoryView>(() => TournamentService.GetHistoryView(), { entries: [] });

  protected time(iso: string): string {
    return this.language.time(iso);
  }
}
