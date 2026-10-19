import { Component, computed, inject, signal } from '@angular/core';
import { RouterLink } from '@angular/router';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { ScheduleBlockView, ScheduleSlotView, ScheduleView, TournamentService } from '@bindings/apps/desktop';
import { Category } from '@bindings/internal/engine';
import { ScheduleMode } from '@bindings/internal/tournament';

import { LanguageService } from '../core/language.service';
import { liveView } from '../core/live-view';
import { TournamentStore } from '../core/tournament.service';
import { CategoryBadge } from '../shared/category-badge';
import { fullNames } from '../shared/names';
import { stageLabel } from '../shared/stage-label';

/** Pixels per minute on the time axis. */
const PX_PER_MIN = 2.4;

const EMPTY: ScheduleView = {
  now: '',
  mode: ScheduleMode.ScheduleAlternate,
  first: '' as Category,
  categories: [],
  courts: 0,
  minRest: 0,
  reserved: [],
  slots: [],
  blocks: [],
  end: null,
  unrested: 0,
};

/**
 * Schedule: the forecast of every match left, court by court, from the
 * estimated durations. The desk changes the order of the blocks (pools,
 * bracket rounds) and, with two categories, how they share the courts.
 * Go computes everything; this screen only lays it out.
 */
@Component({
  selector: 'app-schedule',
  imports: [TranslocoDirective, RouterLink, CategoryBadge],
  templateUrl: './schedule.html',
  styleUrl: './schedule.scss',
})
export class Schedule {
  protected readonly tournament = inject(TournamentStore);
  private readonly transloco = inject(TranslocoService);
  private readonly language = inject(LanguageService);
  protected readonly view = liveView<ScheduleView>(() => TournamentService.GetScheduleView(), EMPTY, 60_000);
  protected readonly modes = ScheduleMode;

  /** Block highlighted in the timeline. */
  protected readonly selected = signal<string | null>(null);

  protected readonly start = computed(() => new Date(this.view().now).getTime());
  protected readonly courts = computed(() => Array.from({ length: this.view().courts }, (_, i) => i + 1));
  protected readonly height = computed(() => {
    const end = this.view().end;
    return end ? Math.max(200, this.minutes(end) * PX_PER_MIN + 40) : 200;
  });

  /** Half-hour marks of the time axis. */
  protected readonly ticks = computed(() => {
    const start = this.start();
    const end = this.view().end ? new Date(this.view().end!).getTime() : start;
    const first = Math.ceil(start / 1_800_000) * 1_800_000;
    const out: { top: number; label: string }[] = [];
    for (let t = first; t <= end + 1_800_000; t += 1_800_000) {
      out.push({ top: ((t - start) / 60_000) * PX_PER_MIN, label: this.time(new Date(t)) });
    }
    return out;
  });

  protected readonly slotsByCourt = computed(() => {
    const out = new Map<number, ScheduleSlotView[]>();
    for (const s of this.view().slots ?? []) {
      out.set(s.court, [...(out.get(s.court) ?? []), s]);
    }
    return out;
  });

  protected reservedFor(court: number): { category: Category; slot: number; manual: boolean } | null {
    const r = (this.view().reserved ?? []).find((x) => x.number === court);
    if (!r) {
      return null;
    }
    const slot = (this.view().categories ?? []).find((c) => c.category === r.category)?.slot ?? 1;
    return { category: r.category, slot, manual: r.manual };
  }

  protected top(s: ScheduleSlotView): number {
    return this.minutes(s.start) * PX_PER_MIN;
  }

  protected slotHeight(s: ScheduleSlotView): number {
    return Math.max(18, (new Date(s.end).getTime() - new Date(s.start).getTime()) / 60_000) * PX_PER_MIN - 3;
  }

  private minutes(value: string | Date): number {
    return (new Date(value).getTime() - this.start()) / 60_000;
  }

  protected time(value: string | Date): string {
    return new Intl.DateTimeFormat(this.language.active, { hour: '2-digit', minute: '2-digit' }).format(new Date(value));
  }

  /** "Pool A" or "Main bracket · round 2". */
  protected blockLabel(b: ScheduleBlockView | ScheduleSlotView): string {
    const t = (key: string, params?: Record<string, unknown>) => this.transloco.translate(key, params);
    const stage = b.entryStage && b.pool ? '' : stageLabel(this.transloco, { name: b.stageName, kind: b.stageKind }) + ' · ';
    return b.pool ? stage + t('schedule.pool', { name: b.pool }) : stage + t('schedule.round', { n: b.round });
  }

  protected sides(s: ScheduleSlotView): string {
    if (!s.match) {
      return this.transloco.translate('schedule.unknownPlayers');
    }
    return `${fullNames(s.match.a)} – ${fullNames(s.match.b)}`;
  }

  protected bibs(s: ScheduleSlotView): string {
    return s.match ? `#${s.match.a.bib || '–'} · #${s.match.b.bib || '–'}` : '';
  }

  // The order of the blocks: moving one sets every block's position, so the
  // order the desk sees is the order the courts follow.
  protected move(index: number, delta: number): void {
    const keys = (this.view().blocks ?? []).map((b) => b.key);
    const [key] = keys.splice(index, 1);
    keys.splice(Math.max(0, Math.min(keys.length, index + delta)), 0, key);
    void this.tournament.setSchedulePins(keys);
  }

  protected toTop(index: number): void {
    this.move(index, -index);
  }

  protected reset(): void {
    void this.tournament.setSchedulePins([]);
  }

  protected readonly manual = computed(() => (this.view().blocks ?? []).some((b) => b.pinned > 0));

  protected setMode(mode: ScheduleMode): void {
    void this.tournament.setScheduleMode(mode);
  }

  protected setFirst(category: Category): void {
    void this.tournament.setScheduleFirst(category);
  }

  protected reserve(court: number, event: Event): void {
    void this.tournament.setCourtCategory(court, (event.target as HTMLSelectElement).value as Category | '');
  }
}
