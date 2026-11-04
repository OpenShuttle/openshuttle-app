import { Component, input } from '@angular/core';
import { TranslocoDirective } from '@jsverse/transloco';
import { ClubBadge } from '@bindings/apps/desktop';

/**
 * Club badges of an entrant, colored by the club's color index from Go
 * (`--color-club-<n>`). A club held by two entrants or more of the same pool
 * stands out, filled, with the count; otherwise it stays pale. Unclubbed
 * players show nothing.
 */
@Component({
  selector: 'app-club-badges',
  imports: [TranslocoDirective],
  template: `<ng-container *transloco="let t">
    @for (c of clubs() ?? []; track c.name) {
      <span
        class="club-badge"
        [class.shared]="c.count > 1"
        [style.--club]="'var(--color-club-' + c.color + ')'"
        [attr.title]="c.count > 1 ? t('draw.clubInPool', { club: c.full, n: c.count }) : c.full"
        ><span class="name">{{ c.name }}</span>
        @if (c.count > 1) {
          <span class="count">×{{ c.count }}</span>
        }
      </span>
    }
  </ng-container>`,
  styles: `
    :host {
      display: inline-flex;
      flex-wrap: wrap;
      gap: 4px;
      min-width: 0;
      max-width: 100%;
    }

    .club-badge {
      display: inline-flex;
      align-items: center;
      gap: 4px;
      min-width: 0;
      max-width: 100%;
      border-radius: 999px;
      padding: 2px 8px;
      font-size: 12px;
      font-weight: 700;
      line-height: 1.3;
      white-space: nowrap;
      color: var(--club);
      background: color-mix(in srgb, var(--club) 10%, var(--color-surface));
      border: 1px solid color-mix(in srgb, var(--club) 30%, var(--color-surface));

      // A dot in the club's color: the pill reads as a club, not a category.
      &::before {
        content: '';
        flex: none;
        width: 7px;
        height: 7px;
        border-radius: 50%;
        background: var(--club);
      }

      &.shared {
        color: #fff;
        background: var(--club);
        border-color: var(--club);
        font-weight: 800;

        &::before {
          background: #fff;
        }
      }
    }

    .name {
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .count {
      flex: none;
      font-family: var(--font-mono);
    }
  `,
})
export class ClubBadges {
  readonly clubs = input.required<ClubBadge[] | null>();
}
