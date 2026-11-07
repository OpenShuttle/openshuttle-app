import { Component, input } from '@angular/core';
import { TranslocoDirective } from '@jsverse/transloco';

/** A trophy for the winner of a match, a cross for its loser. */
@Component({
  selector: 'app-result-icon',
  imports: [TranslocoDirective],
  template: `<ng-container *transloco="let t">
    @if (won()) {
      <svg class="icon won" viewBox="0 0 16 16" role="img" [attr.aria-label]="t('bracket.won')">
        <title>{{ t('bracket.won') }}</title>
        <path
          d="M4 2h8v1.5h2.2c0 2.6-1.3 4.2-3.3 4.5A4 4 0 0 1 9 10v2h2.5v2h-7v-2H7v-2a4 4 0 0 1-1.9-2C3.1 7.7 1.8 6.1 1.8 3.5H4V2Zm0 3H3.4c.2 1 .7 1.6 1.2 1.9A6 6 0 0 1 4 5Zm8 0c0 .7-.2 1.3-.6 1.9.5-.3 1-.9 1.2-1.9H12Z"
        />
      </svg>
    } @else {
      <svg class="icon lost" viewBox="0 0 16 16" role="img" [attr.aria-label]="t('bracket.lost')">
        <title>{{ t('bracket.lost') }}</title>
        <circle cx="8" cy="8" r="6.5" />
        <path d="M5.5 5.5l5 5M10.5 5.5l-5 5" />
      </svg>
    }
  </ng-container>`,
  styles: `
    :host {
      display: inline-flex;
      flex: none;
    }

    .icon {
      width: 1em;
      height: 1em;
    }

    .won {
      fill: var(--color-gold, #d4a017);
    }

    .lost {
      fill: none;
      stroke: var(--color-alert);
      stroke-width: 1.6;
      stroke-linecap: round;
    }
  `,
})
export class ResultIcon {
  readonly won = input.required<boolean>();
}
