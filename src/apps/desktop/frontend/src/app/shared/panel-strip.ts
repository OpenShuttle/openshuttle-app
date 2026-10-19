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
          <span class="round"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M14.5 6.5L9 12l5.5 5.5" /></svg></span>
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
          <span class="round right"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M14.5 6.5L9 12l5.5 5.5" /></svg></span>
        </button>
      }
    </ng-container>
  `,
  styles: `
    .fold {
      padding: 0;
      border: 0;
      background: none;
      cursor: pointer;
    }
    /* Round chevron button, like the one folding the menu. */
    .round {
      width: 26px;
      height: 26px;
      display: grid;
      place-items: center;
      border: 1px solid var(--color-border-strong);
      border-radius: 50%;
      background: var(--color-surface);
      color: var(--color-text-muted);
      box-shadow: 0 1px 4px rgb(0 0 0 / 12%);
      transition: color 0.15s, border-color 0.15s;
    }
    .round svg {
      width: 16px;
      height: 16px;
      fill: none;
      stroke: currentColor;
      stroke-width: 2.2;
      stroke-linecap: round;
      stroke-linejoin: round;
    }
    .round.right svg {
      transform: rotate(180deg);
    }
    .fold:hover .round,
    .strip:hover .round {
      color: var(--color-brand-logo);
      border-color: var(--color-brand-logo);
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
