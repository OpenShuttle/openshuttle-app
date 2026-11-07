import { Component, computed, inject, input } from '@angular/core';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { MatchView, SideView } from '@bindings/apps/desktop';
import { EntrantID } from '@bindings/internal/engine';

import { bracketMatchName, matchWinner, sourceLabel } from './bracket-labels';
import { clubs } from './names';
import { ResultIcon } from './result-icon';
import { WithBibPipe } from './with-bib';

interface Side {
  side: SideView;
  /** Shown instead of the names: a bye, or the match the side comes from. */
  label: string;
  origin: string;
  color: number;
  walkover: boolean;
  /** 'won', 'lost' or '' while the match is not decided. */
  result: string;
}

/**
 * A bracket match card: its name ("1/4 A"), its state, and each side with
 * its clubs, the pool it comes from, and a trophy or a cross once decided.
 * A side not known yet reads "Winner QF A". Actions are projected below.
 */
@Component({
  selector: 'app-bracket-match',
  imports: [TranslocoDirective, ResultIcon, WithBibPipe],
  template: `<ng-container *transloco="let t">
    @let m = match();
    <div class="head">
      <span [attr.title]="m.id">{{ name() }}</span>
      <span class="result">
        @if (m.walkover) {
          <!-- the WO sits on the side that forfeited -->
        } @else if (m.score) {
          {{ score() }}
        } @else if (m.court && !m.endedAt) {
          <span class="pill pill-playing">{{ t('desk.courtN', { n: m.court }) }}</span>
        } @else if (m.endedAt) {
          <span class="pill pill-finished">{{ t('phases.scorePending') }}</span>
        }
      </span>
    </div>
    @for (s of sides(); track $index) {
      <div class="side" [class.won]="s.result === 'won'" [class.lost]="s.result === 'lost'">
        @if (s.origin) {
          <span
            class="origin"
            [class.colored]="!!s.color"
            [style.background]="s.color ? 'var(--color-group-' + s.color + ')' : null"
            >{{ s.origin }}</span
          >
        }
        @if (s.label) {
          <span class="who"><span class="label">{{ s.label }}</span></span>
        } @else {
          <span class="who">
            <span class="names">{{ s.side | withBib }}</span>
            @if (clubs(s.side); as c) {
              <span class="club">{{ c }}</span>
            }
          </span>
        }
        @if (s.walkover) {
          <span class="pill pill-alert wo">WO</span>
        }
        @if (s.result) {
          <app-result-icon class="icon" [won]="s.result === 'won'" />
        }
      </div>
    }
    <ng-content />
  </ng-container>`,
  styles: `
    @use 'mixins' as *;

    :host {
      display: flex;
      flex-direction: column;
      height: 100%;
    }

    .head {
      @include row(8px);
      justify-content: space-between;
      padding: 6px 10px;
      background: var(--color-bg);
      font-size: 12px;
      font-weight: 700;
      color: #33363d;
    }

    .side {
      flex: 1;
      @include row(8px);
      padding: 5px 10px;
      border-top: 1px solid #ecedf0;
      min-height: 40px;
      font-size: 14px;
      font-weight: 600;

      &.won {
        font-weight: 800;
      }

      &.lost {
        color: var(--color-text-muted);
      }
    }

    .who {
      @include stack(1px);
      flex: 1;
      min-width: 0;
    }

    .names,
    .club,
    .label {
      @include ellipsis;
    }

    .club {
      font-size: 12px;
      font-weight: 500;
      color: var(--color-text-muted);
    }

    .label {
      font-style: italic;
      font-weight: 500;
      color: var(--color-text-muted);
    }

    .wo {
      flex: none;
    }

    .icon {
      font-size: 16px;
    }

    // Pool and rank an entrant of a bracket comes from ("A1").
    .origin {
      flex: none;
      padding: 1px 6px;
      border-radius: 4px;
      background: var(--color-surface-muted);
      color: var(--color-text-muted);
      font: 700 11px var(--font-mono);

      &.colored {
        color: #fff;
      }
    }
  `,
})
export class BracketMatch {
  readonly match = input.required<MatchView>();
  /** Pool and rank each entrant comes from ("A1"), and its pool's color. */
  readonly origins = input<Partial<Record<EntrantID, string>> | null | undefined>({});
  readonly originColors = input<Partial<Record<EntrantID, number>> | null | undefined>({});

  private readonly t = inject(TranslocoService);
  protected readonly clubs = clubs;

  protected readonly name = computed(() => bracketMatchName(this.t, this.match()));

  protected readonly score = computed(() =>
    (this.match().score?.sets ?? []).map((s) => `${s.a}-${s.b}`).join(' · '),
  );

  protected readonly sides = computed<Side[]>(() => {
    const m = this.match();
    const winner = matchWinner(m);
    const side = (s: SideView, bye: boolean, from: MatchView['fromA'], walkover: boolean, key: 'a' | 'b'): Side => {
      let label = '';
      if (bye) {
        label = this.t.translate('phases.bye');
      } else if (!s.entrant) {
        label = from ? sourceLabel(this.t, from) : this.t.translate('phases.tbd');
      }
      return {
        side: s,
        label,
        origin: (s.entrant && this.origins()?.[s.entrant]) || '',
        color: (s.entrant && this.originColors()?.[s.entrant]) || 0,
        walkover,
        result: winner && s.entrant ? (winner === key ? 'won' : 'lost') : '',
      };
    };
    return [side(m.a, m.byeA, m.fromA, m.walkoverA, 'a'), side(m.b, m.byeB, m.fromB, m.walkoverB, 'b')];
  });
}
