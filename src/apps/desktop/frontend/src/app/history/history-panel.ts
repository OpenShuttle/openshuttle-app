import { Component, inject, output } from '@angular/core';
import { TranslocoDirective } from '@jsverse/transloco';
import { HistoryView, TournamentService } from '@bindings/apps/desktop';

import { ActionLabels } from '../core/action-label';
import { LanguageService } from '../core/language.service';
import { liveView } from '../core/live-view';

/** Side panel listing every action, newest first; undone ones struck out. */
@Component({
  selector: 'app-history-panel',
  imports: [TranslocoDirective],
  template: `
    <aside *transloco="let t" class="drawer" (keydown.escape)="closed.emit()">
      <header>
        <h2>{{ t('historyPanel.title') }}</h2>
        <button type="button" class="close" (click)="closed.emit()" [attr.aria-label]="t('common.dismiss')">×</button>
      </header>
      <ol>
        @for (e of view().entries; track e.seq) {
          <li [class.undone]="e.undone">
            <span class="time">{{ time(e.at) }}</span>
            <span class="label">{{ labels.label(e) }}</span>
            @if (e.operator) {
              <span class="operator">{{ e.operator }}</span>
            }
          </li>
        }
      </ol>
    </aside>
  `,
  styles: `
    .drawer {
      position: fixed;
      top: 64px;
      right: 0;
      bottom: 0;
      z-index: 5;
      width: 400px;
      display: flex;
      flex-direction: column;
      background: var(--color-surface);
      border-left: 1px solid var(--color-border);
      box-shadow: -10px 0 30px rgb(0 0 0 / 8%);
    }
    header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 16px 18px;
      border-bottom: 1px solid var(--color-border);
    }
    h2 {
      margin: 0;
      font-size: 18px;
      font-weight: 800;
    }
    .close {
      border: none;
      background: none;
      font-size: 22px;
    }
    ol {
      list-style: none;
      margin: 0;
      padding: 8px 18px;
      overflow: auto;
    }
    li {
      display: grid;
      grid-template-columns: 68px minmax(0, 1fr);
      gap: 2px 8px;
      padding: 8px 0;
      border-bottom: 1px solid var(--color-border);
      font-size: 14px;
    }
    .time {
      font: 600 12px var(--font-mono);
      color: var(--color-text-muted);
      padding-top: 2px;
    }
    .operator {
      grid-column: 2;
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
  readonly closed = output<void>();
  protected readonly labels = inject(ActionLabels);
  private readonly language = inject(LanguageService);
  protected readonly view = liveView<HistoryView>(() => TournamentService.GetHistoryView(), { entries: [] });

  protected time(iso: string): string {
    return new Intl.DateTimeFormat(this.language.active, { hour: '2-digit', minute: '2-digit' }).format(new Date(iso));
  }
}
