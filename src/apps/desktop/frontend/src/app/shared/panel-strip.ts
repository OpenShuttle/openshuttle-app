import { Component, inject, input } from '@angular/core';
import { TranslocoDirective } from '@jsverse/transloco';

import { Panels } from '../core/panels.service';

/**
 * The button folding a right panel, and the thin strip it leaves when folded:
 * the panel's title written vertically, a click to open it again.
 */
@Component({
  selector: 'app-panel-toggle',
  imports: [TranslocoDirective],
  template: `
    <ng-container *transloco="let t">
      @if (panels.collapsed(key())) {
        <button type="button" class="strip" [attr.aria-label]="t('panels.expand', { name: title() })" (click)="panels.toggle(key())">
          <span class="arrow">«</span>
          <span class="vertical">{{ title() }}</span>
        </button>
      } @else {
        <button
          type="button"
          class="fold"
          [attr.aria-label]="t('panels.collapse', { name: title() })"
          [attr.title]="t('panels.collapse', { name: title() })"
          (click)="panels.toggle(key())"
        >
          »
        </button>
      }
    </ng-container>
  `,
  styles: `
    .fold {
      width: 28px;
      height: 28px;
      border: 1px solid var(--color-border);
      border-radius: var(--radius-sm);
      background: var(--color-surface);
      font-weight: 800;
      color: var(--color-text-muted);
      cursor: pointer;
    }
    .strip {
      width: 44px;
      min-height: 160px;
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: 10px;
      padding: 10px 0;
      border: 1px solid var(--color-border);
      border-radius: var(--radius-md);
      background: var(--color-surface);
      color: var(--color-text-muted);
      font-weight: 800;
      cursor: pointer;
    }
    .vertical {
      writing-mode: vertical-rl;
      font-size: 13px;
      white-space: nowrap;
    }
  `,
})
export class PanelToggle {
  readonly key = input.required<string>();
  readonly title = input.required<string>();
  protected readonly panels = inject(Panels);
}
