import { Component, ElementRef, HostListener, afterEveryRender, computed, effect, inject, signal, untracked } from '@angular/core';
import { RouterLink } from '@angular/router';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { ScheduleGroupView, ScheduleSlotView, ScheduleView, TournamentService } from '@bindings/apps/desktop';
import { Category } from '@bindings/internal/engine';
import { MatchID, ScheduleMode } from '@bindings/internal/tournament';

import { LanguageService } from '../core/language.service';
import { Panels } from '../core/panels.service';
import { liveView } from '../core/live-view';
import { TournamentStore } from '../core/tournament.service';
import { CategoryBadge } from '../shared/category-badge';
import { fullNames } from '../shared/names';
import { PanelToggle } from '../shared/panel-strip';
import { roundLabel } from '../shared/match-stage';
import { stageLabel } from '../shared/stage-label';

/** Pixels per minute on the time axis. */
const PX_PER_MIN = 2.4;

/** Pointer travel before a press becomes a drag, in pixels. */
const DRAG_THRESHOLD = 5;

/** What a drop lands on: a planned slot (its index in the view the drag started from), or the end of the order. */
type DropTarget = { slot: number } | { end: true };

function sameTarget(a: DropTarget | null, b: DropTarget | null): boolean {
  return a === b || (!!a && !!b && ('slot' in a ? 'slot' in b && a.slot === b.slot : 'end' in b));
}

/** The time axis starts with the first match on court, or now. */
function axisStart(v: ScheduleView): number {
  return Math.min(new Date(v.now).getTime(), ...(v.slots ?? []).map((s) => new Date(s.start).getTime()));
}

const EMPTY: ScheduleView = {
  now: '',
  mode: ScheduleMode.ScheduleAlternate,
  first: '' as Category,
  categories: [],
  courts: 0,
  minRest: 0,
  reserved: [],
  slots: [],
  validatedAt: null,
  groups: [],
  end: null,
  unrested: 0,
};

/**
 * Schedule: every match court by court, played ones greyed, the others at
 * their estimated times. Before validation the desk drags a match to move
 * its whole block (pool or bracket round); once validated, matches move one
 * by one. Go computes everything; this screen only lays it out.
 */
@Component({
  selector: 'app-schedule',
  imports: [TranslocoDirective, RouterLink, CategoryBadge, PanelToggle],
  templateUrl: './schedule.html',
  styleUrl: './schedule.scss',
})
export class Schedule {
  protected readonly tournament = inject(TournamentStore);
  protected readonly panels = inject(Panels);
  private readonly transloco = inject(TranslocoService);
  private readonly language = inject(LanguageService);
  private readonly live = liveView<ScheduleView>(() => TournamentService.GetScheduleView(), EMPTY, 60_000);
  /** While dragging, the schedule as it would be after the drop (computed by Go, not saved). */
  private readonly preview = signal<ScheduleView | null>(null);
  protected readonly view = computed(() => this.preview() ?? this.live());
  protected readonly modes = ScheduleMode;
  protected readonly fullNames = fullNames;

  /** A second click confirms an action that drops the matches moved by hand. */
  protected readonly confirming = signal<'recompute' | 'unvalidate' | null>(null);

  /** The time axis starts with the first match on court, or now. */
  protected readonly start = computed(() => axisStart(this.view()));
  protected readonly courts = computed(() => Array.from({ length: this.view().courts }, (_, i) => i + 1));
  protected readonly height = computed(() => {
    const ends = (this.view().slots ?? []).map((s) => this.minutes(s.end));
    return Math.max(200, Math.max(0, ...ends) * PX_PER_MIN + 40);
  });
  /** Matches still to play (stand-ins included). */
  protected readonly planned = computed(() => (this.view().slots ?? []).filter((s) => s.state === 'planned').length);

  /** Where "now" sits on the time axis. */
  protected readonly nowTop = computed(() => this.minutes(this.view().now) * PX_PER_MIN);

  /** Half-hour marks of the time axis. */
  protected readonly ticks = computed(() => {
    const start = this.start();
    const end = start + (this.height() / PX_PER_MIN) * 60_000;
    const out: { top: number; label: string }[] = [];
    for (let t = Math.ceil(start / 1_800_000) * 1_800_000; t <= end; t += 1_800_000) {
      out.push({ top: ((t - start) / 60_000) * PX_PER_MIN, label: this.time(new Date(t)) });
    }
    return out;
  });

  /**
   * Slots per court, each with its index in the view and the key that
   * follows it from one view to the next (its match, or for a stand-in its
   * block, round and rank), so a moved match glides to its new place.
   */
  protected readonly slotsByCourt = computed(() => {
    const out = new Map<number, { slot: ScheduleSlotView; index: number; key: string }[]>();
    const seen = new Map<string, number>();
    (this.view().slots ?? []).forEach((slot, index) => {
      let key = slot.match?.id as string | undefined;
      if (!key) {
        const block = `${slot.key}#${slot.round}`;
        seen.set(block, (seen.get(block) ?? 0) + 1);
        key = `${block}#${seen.get(block)}`;
      }
      out.set(slot.court, [...(out.get(slot.court) ?? []), { slot, index, key }]);
    });
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

  /** "Pool A · round 2", "Main bracket · Quarter-finals" or "Places 5–8"; a group: "Pool A". */
  protected label(b: ScheduleSlotView | ScheduleGroupView): string {
    const t = (key: string, params?: Record<string, unknown>) => this.transloco.translate(key, params);
    const stage = stageLabel(this.transloco, { name: b.stageName, kind: b.stageKind });
    if (!('round' in b)) {
      return b.pool ? (b.entryStage ? '' : stage + ' · ') + t('schedule.pool', { name: b.pool }) : stage;
    }
    if (b.pool) {
      return (b.entryStage ? '' : stage + ' · ') + t('schedule.poolRound', { name: b.pool, n: b.round });
    }
    // A bracket match known: its round and places; a stand-in: the round by its size.
    const r = b.match ? roundLabel(b.match) : roundLabel({ placeFrom: 1, placeTo: 0, roundSize: b.roundSize });
    return `${stage} · ${t(r.key, r.params)}`;
  }

  protected bibs(s: ScheduleSlotView): string {
    return s.match ? `#${s.match.a.bib || '–'} · #${s.match.b.bib || '–'}` : '';
  }

  protected title(s: ScheduleSlotView): string {
    const t = (key: string, params?: Record<string, unknown>) => this.transloco.translate(key, params);
    const sides = s.match
      ? `${fullNames(s.match.a)} (${t('schedule.left', { n: s.leftA })}) – ${fullNames(s.match.b)} (${t('schedule.left', { n: s.leftB })})`
      : t('schedule.unknownPlayers');
    return `${this.time(s.start)} – ${this.time(s.end)} · ${this.label(s)} · ${sides}`;
  }

  // Validating fixes the order; recomputing or going back to blocks drops
  // the matches moved by hand, so they take a second click.
  protected validate(): void {
    if (this.view().validatedAt && this.confirming() !== 'recompute') {
      this.confirming.set('recompute');
      return;
    }
    this.confirming.set(null);
    void this.tournament.validateSchedule();
  }

  protected unvalidate(): void {
    if (this.confirming() !== 'unvalidate') {
      this.confirming.set('unvalidate');
      return;
    }
    this.confirming.set(null);
    void this.tournament.unvalidateSchedule();
  }

  /**
   * Drag and drop with pointer events (HTML5 drag and drop is unreliable in
   * webviews). While dragging, the timeline shows the preview of the drop, so
   * the desk sees what shifts; the target is found on the schedule the drag
   * started from, so the preview moving under the pointer never changes it.
   */
  protected readonly dragging = signal<ScheduleSlotView | null>(null);
  /** While dragging, what the drop does, said above the timeline. */
  protected readonly dropInfo = signal<{ slot: ScheduleSlotView; text: string } | null>(null);
  /** Start of each waiting match when the drag began, to show how far the preview shifts it. */
  private baseStarts = new Map<string, number>();
  private drag: { slot: ScheduleSlotView; base: ScheduleView; x: number; y: number; moved: boolean; target: DropTarget | null } | null =
    null;
  private previewSeq = 0;

  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  /** Where each match was drawn, within the grid (scrolling does not move it). */
  private positions = new Map<string, { x: number; y: number }>();

  constructor() {
    // A new schedule from Go replaces the preview of the last drop.
    effect(() => {
      this.live();
      untracked(() => !this.drag && this.preview.set(null));
    });
    afterEveryRender(() => this.glide());
  }

  /**
   * Every match moved by the last render glides from where it was drawn to
   * its new place, from one court to another included (FLIP): while dragging,
   * the desk sees the others take the freed places and the moved matches
   * take theirs.
   */
  private glide(): void {
    const next = new Map<string, { x: number; y: number }>();
    for (const el of this.host.nativeElement.querySelectorAll<HTMLElement>('[data-flip]')) {
      const lane = el.offsetParent as HTMLElement | null;
      const at = { x: el.offsetLeft + (lane?.offsetLeft ?? 0), y: el.offsetTop + (lane?.offsetTop ?? 0) };
      const key = el.dataset['flip'] ?? '';
      next.set(key, at);
      const was = this.positions.get(key);
      if (!was || (was.x === at.x && was.y === at.y)) {
        continue;
      }
      // Start from where it is seen, halfway through a glide included.
      let dx = was.x - at.x;
      let dy = was.y - at.y;
      const running = el.getAnimations();
      if (running.length > 0) {
        const m = new DOMMatrix(getComputedStyle(el).transform);
        dx += m.e;
        dy += m.f;
        running.forEach((a) => a.cancel());
      }
      el.animate([{ transform: `translate(${dx}px, ${dy}px)` }, { transform: 'translate(0, 0)' }], {
        duration: 280,
        easing: 'cubic-bezier(0.2, 0.8, 0.2, 1)',
      });
    }
    this.positions = next;
  }

  protected onSlotDown(event: PointerEvent, slot: ScheduleSlotView): void {
    if (slot.movable && event.button === 0) {
      const base = this.live();
      this.drag = { slot, base, x: event.clientX, y: event.clientY, moved: false, target: null };
      this.baseStarts = new Map((base.slots ?? []).filter((s) => s.match).map((s) => [s.match!.id, new Date(s.start).getTime()]));
    }
  }

  @HostListener('document:pointermove', ['$event'])
  protected onPointerMove(event: PointerEvent): void {
    const drag = this.drag;
    if (!drag || (!drag.moved && Math.hypot(event.clientX - drag.x, event.clientY - drag.y) < DRAG_THRESHOLD)) {
      return;
    }
    drag.moved = true;
    event.preventDefault();
    this.dragging.set(drag.slot);
    const target = this.targetAt(drag.base, event.clientX, event.clientY);
    if (!this.dropInfo() || !sameTarget(target, drag.target)) {
      drag.target = target;
      void this.showPreview(drag.slot, drag.base, target);
      this.dropInfo.set({ slot: drag.slot, text: this.dropText(drag.slot, drag.base, target) });
    }
  }

  @HostListener('document:pointerup')
  protected onPointerUp(): void {
    const drag = this.drag;
    this.drag = null;
    this.dragging.set(null);
    this.dropInfo.set(null);
    this.previewSeq++;
    if (drag?.moved && drag.target) {
      void this.drop(drag.slot, drag.base, drag.target);
    } else {
      this.preview.set(null);
    }
  }

  /**
   * The slot of the starting schedule under the pointer: on the court under
   * it, the first match not over at that time (a gap means before the next
   * one); below the last one, the end of the order. Played and playing
   * matches take nothing before them.
   */
  private targetAt(base: ScheduleView, x: number, y: number): DropTarget | null {
    const lane = document.elementFromPoint(x, y)?.closest<HTMLElement>('[data-lane]');
    if (!lane) {
      return null;
    }
    const court = Number(lane.dataset['lane']);
    const start = axisStart(base);
    const at = start + ((y - lane.getBoundingClientRect().top) / PX_PER_MIN) * 60_000;
    const slots = base.slots ?? [];
    let found = -1;
    slots.forEach((s, i) => {
      if (s.court === court && new Date(s.end).getTime() > at && (found < 0 || new Date(s.start) < new Date(slots[found].start))) {
        found = i;
      }
    });
    if (found < 0) {
      return { end: true };
    }
    return slots[found].state === 'planned' ? { slot: found } : null;
  }

  /** What the drop does: before validation the block moves before the target's block; after, the match before the target match. */
  private move(slot: ScheduleSlotView, base: ScheduleView, target: DropTarget): { block: string; before: string } | { match: MatchID; before: MatchID | '' } | null {
    const on = 'slot' in target ? (base.slots ?? [])[target.slot] : null;
    if (base.validatedAt) {
      return slot.match ? { match: slot.match.id, before: (on?.match?.id ?? '') as MatchID | '' } : null;
    }
    const before = on?.movable ? on.key : '';
    return before === slot.key ? null : { block: slot.key, before };
  }

  /** What the drop does, in words. */
  private dropText(slot: ScheduleSlotView, base: ScheduleView, target: DropTarget | null): string {
    const t = (key: string, params?: Record<string, unknown>) => this.transloco.translate(key, params);
    const m = target && this.move(slot, base, target);
    if (!m || !target) {
      return t('schedule.dropNone');
    }
    const on = 'slot' in target ? (base.slots ?? [])[target.slot] : null;
    if ('match' in m) {
      return on?.match ? t('schedule.dropBefore', { label: this.label(on), court: on.court, time: this.time(on.start) }) : t('schedule.dropLast');
    }
    const label = this.blockLabel(slot);
    return m.before && on ? t('schedule.dropBlockBefore', { label, before: this.blockLabel(on) }) : t('schedule.dropBlockLast', { label });
  }

  /** A block: its pool ("Pool A") or its bracket round. */
  private blockLabel(s: ScheduleSlotView): string {
    if (!s.pool) {
      return this.label(s);
    }
    const { round: _, ...group } = s;
    return this.label(group as unknown as ScheduleGroupView);
  }

  /** Where the preview puts the dragged match: its court and start. */
  protected readonly landing = computed(() => {
    const d = this.dragging();
    const p = this.preview();
    return d?.match && p?.validatedAt ? ((p.slots ?? []).find((s) => s.state === 'planned' && s.match?.id === d.match!.id) ?? null) : null;
  });

  /** How far the preview moves a match from where it was when the drag began, in minutes (0: not moved). */
  protected shift(s: ScheduleSlotView): number {
    const before = this.preview() && s.state === 'planned' && s.match ? this.baseStarts.get(s.match.id) : undefined;
    return before === undefined ? 0 : Math.round((new Date(s.start).getTime() - before) / 60_000);
  }

  private async showPreview(slot: ScheduleSlotView, base: ScheduleView, target: DropTarget | null): Promise<void> {
    const seq = ++this.previewSeq;
    const m = target && this.move(slot, base, target);
    if (!m) {
      this.preview.set(null);
      return;
    }
    try {
      const v = await ('block' in m
        ? TournamentService.PreviewScheduledBlockMove(m.block, m.before)
        : TournamentService.PreviewScheduledMatchMove(m.match, m.before as MatchID));
      if (seq === this.previewSeq) {
        this.preview.set(v);
      }
    } catch (err) {
      console.error('schedule preview', err);
      if (seq === this.previewSeq) {
        this.preview.set(null);
      }
    }
  }

  private async drop(slot: ScheduleSlotView, base: ScheduleView, target: DropTarget): Promise<void> {
    const m = this.move(slot, base, target);
    const ok =
      !!m && (await ('block' in m ? this.tournament.moveScheduledBlock(m.block, m.before) : this.tournament.moveScheduledMatch(m.match, m.before)));
    // Keep showing the result until the live view catches up, never the old order.
    const seq = this.previewSeq;
    const fresh = ok ? await TournamentService.GetScheduleView() : null;
    if (seq === this.previewSeq) {
      this.preview.set(fresh);
    }
  }

  /** Slots carried by the drag: the match, or its whole block before validation. */
  protected carried(s: ScheduleSlotView): boolean {
    const d = this.dragging();
    if (!d || s.state !== 'planned') {
      return false;
    }
    return this.view().validatedAt ? s.match?.id === d.match?.id : s.key === d.key;
  }

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
