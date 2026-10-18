import { Component, computed, inject, input, output, signal } from '@angular/core';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { PresetService, StageView } from '@bindings/apps/desktop';
import { CrossPool, Tiebreak, WinCondition, WinConditionKind } from '@bindings/internal/engine';
import { RouteKind, RouteRule, StageConfig, StageKind } from '@bindings/internal/tournament';

import { AppPresets } from '../core/presets.service';
import { stageLabel, winSummary } from '../shared/stage-label';

const MAX_POOLS = 16;
const POOL_SIZE_CHOICES = [2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12];
const DEFAULT_SETS: WinCondition = { kind: WinConditionKind.WinBySets, setsToWin: 2, pointsPerSet: 21, cap: 30 };
const DEFAULT_TIMED: WinCondition = { kind: WinConditionKind.WinByTime, minutes: 12 };
const POINT_CHOICES = [11, 15, 21];

/** Canvas geometry of the stage graph, in pixels. */
const NODE_W = 190;
const NODE_H = 140;
const COL_GAP = 140;
const ROW_GAP = 26;
const PAD = 16;
const OUTPUT_W = 170;
/** Height of the lane under the stages that early exits run along. */
const LANE_H = 44;

/** Selects the last stage, whatever its id: the one just added. */
export const LAST_STAGE = '__last';

interface GraphNode {
  stage: StageView;
  x: number;
  y: number;
}

interface GraphEdge {
  d: string;
  label: string;
  x: number;
  y: number;
}

/**
 * Edits a format: the stages as a graph (phase by phase, linked by routing),
 * the preview for the entrants, and the selected stage in a side panel. The
 * host applies the changes: to a tournament (through Go) or to a preset
 * being edited. The editor computes nothing about the tournament: stages
 * come with their preview from Go.
 */
@Component({
  selector: 'app-format-editor',
  imports: [TranslocoDirective],
  templateUrl: './format-editor.html',
  styleUrl: './format-editor.scss',
})
export class FormatEditor {
  readonly stages = input.required<StageView[]>();
  readonly timedAllowed = input(true);
  readonly ranked = input(0);
  readonly ties = input(false);
  readonly entrants = input(0);
  readonly courts = input(0);
  /** Built-in presets are shown, never changed. */
  readonly readonly = input(false);

  /** The whole format after a change to a stage. */
  readonly changed = output<StageConfig[]>();
  readonly added = output<StageKind>();
  readonly removed = output<string>();

  protected readonly presets = inject(AppPresets);
  private readonly transloco = inject(TranslocoService);
  protected readonly maxPools = MAX_POOLS;
  protected readonly poolSizeChoices = POOL_SIZE_CHOICES;
  protected readonly kinds = WinConditionKind;
  protected readonly stageKinds = StageKind;
  protected readonly routeKinds = RouteKind;
  protected readonly crossPools = CrossPool;
  protected readonly pointChoices = POINT_CHOICES;
  protected readonly nodeW = NODE_W;
  protected readonly nodeH = NODE_H;
  protected readonly outputW = OUTPUT_W;

  /** Selected stage ID; the entry stage by default. */
  readonly selectedID = signal('');
  protected readonly selected = computed(() => {
    const stages = this.stages();
    const id = this.selectedID();
    return (id === LAST_STAGE ? stages[stages.length - 1] : stages.find((s) => s.id === id)) ?? stages[0] ?? null;
  });

  /** The save-as-preset form of the side panel, when open. */
  protected readonly saving = signal<'win' | 'tiebreaks' | null>(null);
  protected readonly presetName = signal('');

  // Graph layout: one column per phase, the output last.
  private readonly phases = computed(() => Math.max(1, ...this.stages().map((s) => s.phase)));
  /** Height of the stage rows, the exit lane below them excluded. */
  private readonly rowsHeight = computed(() => {
    const perPhase = new Map<number, number>();
    for (const s of this.stages()) {
      perPhase.set(s.phase, (perPhase.get(s.phase) ?? 0) + 1);
    }
    const rows = Math.max(1, ...perPhase.values());
    return Math.max(220, rows * (NODE_H + ROW_GAP) - ROW_GAP + 2 * PAD);
  });
  /** Stages before the last phase that give final places exit along a lane under the graph. */
  private readonly hasLane = computed(() => this.stages().some((s) => s.placeFrom > 0 && s.phase < this.phases()));
  protected readonly canvasHeight = computed(() => this.rowsHeight() + (this.hasLane() ? LANE_H : 0));
  protected readonly canvasWidth = computed(() => PAD + this.phases() * (NODE_W + COL_GAP) + OUTPUT_W + PAD);
  protected readonly outputX = computed(() => PAD + this.phases() * (NODE_W + COL_GAP));
  protected readonly outputY = computed(() => this.rowsHeight() / 2 - NODE_H / 2);

  protected readonly nodes = computed<GraphNode[]>(() => {
    const h = this.rowsHeight();
    const byPhase = new Map<number, StageView[]>();
    for (const s of this.stages()) {
      byPhase.set(s.phase, [...(byPhase.get(s.phase) ?? []), s]);
    }
    const out: GraphNode[] = [];
    for (const [phase, stages] of byPhase) {
      const top = h / 2 - (stages.length * (NODE_H + ROW_GAP) - ROW_GAP) / 2;
      stages.forEach((stage, i) =>
        out.push({ stage, x: PAD + (phase - 1) * (NODE_W + COL_GAP), y: top + i * (NODE_H + ROW_GAP) }),
      );
    }
    return out;
  });

  protected readonly edges = computed<GraphEdge[]>(() => {
    const nodes = new Map(this.nodes().map((n) => [n.stage.id, n]));
    const out: GraphEdge[] = [];
    const exit = { x: this.outputX(), y: this.outputY() + NODE_H / 2 };
    for (const n of this.nodes()) {
      const from = { x: n.x + NODE_W, y: n.y + NODE_H / 2 };
      const counts = new Map<string, number>();
      (n.stage.routing ?? []).forEach((r, i) => counts.set(r.to, (counts.get(r.to) ?? 0) + (n.stage.rules?.[i] ?? 0)));
      for (const [target, count] of counts) {
        const to = nodes.get(target);
        if (to) {
          out.push(edge(from, { x: to.x, y: to.y + NODE_H / 2 }, this.transloco.translate('format.edgeCount', { n: count })));
        }
      }
      if (n.stage.placeFrom > 0) {
        const label = places(n.stage.placeFrom, n.stage.placeTo);
        out.push(
          n.stage.phase < this.phases()
            ? laneEdge(from, exit, this.rowsHeight() + LANE_H / 2 - PAD / 2, label)
            : edge(from, exit, label),
        );
      }
    }
    return out;
  });

  /** Stages some routing rule sends entrants to. */
  protected readonly fed = computed(() => new Set(this.stages().flatMap((s) => (s.routing ?? []).map((r) => r.to))));

  protected readonly totalMatches = computed(() => this.stages().reduce((n, s) => n + s.matches, 0));

  protected select(id: string): void {
    this.selectedID.set(id);
  }

  protected label(stage: { name: string; kind: string }): string {
    return stageLabel(this.transloco, stage);
  }

  protected win(win: WinCondition): string {
    return winSummary(this.transloco, win);
  }

  protected places(from: number, to: number): string {
    return places(from, to);
  }

  /** "3 pools of 4" or "5 pools (5, 5, 4, 4, 4)" or "pools of 4 or 3". */
  protected poolsTitle(stage: StageView): string {
    const sizes = stage.sizes ?? [];
    if (sizes.length === 0) {
      return (stage.poolSizes ?? []).length
        ? this.transloco.translate('format.poolsOfSizes', { sizes: this.sizesText(stage) })
        : this.transloco.translate('format.poolsN', { n: stage.pools ?? 0 });
    }
    return sizes.every((s) => s === sizes[0])
      ? this.transloco.translate('format.poolsOf', { n: sizes.length, size: sizes[0] })
      : this.transloco.translate('format.poolsSizes', { n: sizes.length, sizes: sizes.join(', ') });
  }

  /** "4, 3 or 5", in order of preference. */
  protected sizesText(stage: StageView): string {
    const sizes = stage.poolSizes ?? [];
    if (sizes.length < 2) {
      return sizes.join('');
    }
    return this.transloco.translate('format.orList', {
      first: sizes.slice(0, -1).join(', '),
      last: sizes[sizes.length - 1],
    });
  }

  /** Stages after this one: where its entrants can go. */
  protected laterStages(stage: StageView): StageView[] {
    const stages = this.stages();
    return stages.slice(stages.findIndex((s) => s.id === stage.id) + 1);
  }

  // Edits: each emits the whole format.

  protected edit(stage: StageView, change: Partial<StageConfig>): void {
    if (this.readonly()) {
      return;
    }
    this.changed.emit(this.stages().map((s) => (s.id === stage.id ? { ...config(s), ...change } : config(s))));
  }

  protected setKind(stage: StageView, kind: StageKind): void {
    if (kind === stage.kind) {
      return;
    }
    this.edit(
      stage,
      kind === StageKind.StagePools
        ? {
            kind,
            pools: 0,
            poolSizes: [4, 3, 5],
            tiebreaks: [...(this.presets.value().tiebreaks?.[0]?.tiebreaks ?? [])],
            crossPool: CrossPool.CrossPoolSameMatches,
            fullPlacement: false,
          }
        : { kind, pools: 0, poolSizes: [], tiebreaks: [], fullPlacement: true },
    );
  }

  /** Pools by count, or by sizes in order of preference. */
  protected setPoolMode(stage: StageView, bySizes: boolean): void {
    if (bySizes === (stage.poolSizes ?? []).length > 0) {
      return;
    }
    this.edit(stage, bySizes ? { pools: 0, poolSizes: [4, 3, 5] } : { pools: Math.max(1, (stage.sizes ?? []).length || 2), poolSizes: [] });
  }

  protected movePoolSize(stage: StageView, index: number, step: -1 | 1): void {
    const sizes = [...(stage.poolSizes ?? [])];
    const target = index + step;
    if (target >= 0 && target < sizes.length) {
      [sizes[index], sizes[target]] = [sizes[target], sizes[index]];
      this.edit(stage, { poolSizes: sizes });
    }
  }

  protected removePoolSize(stage: StageView, index: number): void {
    const sizes = (stage.poolSizes ?? []).filter((_, i) => i !== index);
    if (sizes.length > 0) {
      this.edit(stage, { poolSizes: sizes });
    }
  }

  protected addPoolSize(stage: StageView, event: Event): void {
    const select = event.target as HTMLSelectElement;
    const size = Number(select.value);
    select.value = '';
    if (size && !(stage.poolSizes ?? []).includes(size)) {
      this.edit(stage, { poolSizes: [...(stage.poolSizes ?? []), size] });
    }
  }

  protected setWinKind(stage: StageView, kind: WinConditionKind): void {
    if (kind !== stage.win.kind) {
      this.edit(stage, { win: kind === WinConditionKind.WinByTime ? DEFAULT_TIMED : DEFAULT_SETS });
    }
  }

  protected stepWin(stage: StageView, field: 'setsToWin' | 'cap' | 'minutes', delta: number): void {
    this.edit(stage, { win: { ...stage.win, [field]: (stage.win[field] ?? 0) + delta } });
  }

  protected setPoints(stage: StageView, points: number): void {
    this.edit(stage, { win: { ...stage.win, pointsPerSet: points } });
  }

  protected applyWinPreset(stage: StageView, event: Event): void {
    const select = event.target as HTMLSelectElement;
    const preset = (this.presets.value().win ?? []).find((p) => p.id === select.value);
    select.value = '';
    if (preset) {
      this.edit(stage, { win: preset.win });
    }
  }

  protected applyTiebreakPreset(stage: StageView, event: Event): void {
    const select = event.target as HTMLSelectElement;
    const preset = (this.presets.value().tiebreaks ?? []).find((p) => p.id === select.value);
    select.value = '';
    if (preset) {
      this.edit(stage, { tiebreaks: [...(preset.tiebreaks ?? [])] });
    }
  }

  protected moveTiebreak(stage: StageView, index: number, step: -1 | 1): void {
    const tiebreaks = [...(stage.tiebreaks ?? [])];
    const target = index + step;
    if (target >= 0 && target < tiebreaks.length) {
      [tiebreaks[index], tiebreaks[target]] = [tiebreaks[target], tiebreaks[index]];
      this.edit(stage, { tiebreaks });
    }
  }

  protected tiebreaks(stage: StageView): Tiebreak[] {
    return stage.tiebreaks ?? [];
  }

  // Routing rules.

  protected addRule(stage: StageView): void {
    const to = this.laterStages(stage)[0]?.id;
    if (to) {
      const rule: RouteRule = { kind: RouteKind.RouteRanks, to, ranks: [1, 2] };
      this.edit(stage, { routing: [...(stage.routing ?? []), rule] });
    }
  }

  protected removeRule(stage: StageView, index: number): void {
    this.edit(stage, { routing: (stage.routing ?? []).filter((_, i) => i !== index) });
  }

  protected setRule(stage: StageView, index: number, change: Partial<RouteRule>): void {
    const routing = (stage.routing ?? []).map((r, i) => (i === index ? { ...r, ...change } : r));
    this.edit(stage, { routing });
  }

  protected setRuleKind(stage: StageView, index: number, event: Event): void {
    const kind = (event.target as HTMLSelectElement).value as RouteKind;
    const to = stage.routing?.[index]?.to ?? '';
    const defaults: Record<string, RouteRule> = {
      [RouteKind.RouteRanks]: { kind, to, ranks: [1, 2] },
      [RouteKind.RouteBestOfRank]: { kind, to, rank: 3, count: 1 },
      [RouteKind.RouteBest]: { kind, to, count: 8 },
      [RouteKind.RouteRest]: { kind, to },
    };
    this.setRule(stage, index, { ranks: [], rank: 0, count: 0, ...defaults[kind] });
  }

  protected setRuleTarget(stage: StageView, index: number, event: Event): void {
    this.setRule(stage, index, { to: (event.target as HTMLSelectElement).value });
  }

  /** First and last rank of a "ranks" rule, edited as a range. */
  protected rangeOf(rule: RouteRule): { from: number; to: number } {
    const ranks = rule.ranks ?? [];
    return { from: ranks[0] ?? 1, to: ranks[ranks.length - 1] ?? 1 };
  }

  protected stepRange(stage: StageView, index: number, end: 'from' | 'to', delta: number): void {
    const range = this.rangeOf(stage.routing![index]);
    const next = { ...range, [end]: range[end] + delta };
    if (next.from < 1 || next.to < next.from) {
      return;
    }
    const ranks = Array.from({ length: next.to - next.from + 1 }, (_, i) => next.from + i);
    this.setRule(stage, index, { ranks });
  }

  protected stepRule(stage: StageView, index: number, field: 'rank' | 'count', delta: number): void {
    const rule = stage.routing![index];
    const value = (rule[field] ?? 0) + delta;
    if (value >= 1) {
      this.setRule(stage, index, { [field]: value });
    }
  }

  protected openSave(kind: 'win' | 'tiebreaks'): void {
    this.presetName.set('');
    this.saving.set(kind);
  }

  protected async savePreset(stage: StageView): Promise<void> {
    const name = this.presetName();
    const done =
      this.saving() === 'win'
        ? await this.presets.run(() => PresetService.SaveWinPreset('', name, stage.win))
        : await this.presets.run(() => PresetService.SaveTiebreakPreset('', name, stage.tiebreaks ?? []));
    if (done) {
      this.saving.set(null);
    }
  }

  protected tiebreakPresetLabel(p: { key: string; name: string }): string {
    return p.name || this.transloco.translate('presets.tiebreaks.' + p.key);
  }

  protected value(event: Event): string {
    return (event.target as HTMLInputElement).value;
  }
}

/** The editable part of a stage. */
export function config(s: StageView): StageConfig {
  return {
    id: s.id,
    name: s.name,
    kind: s.kind,
    win: s.win,
    pools: s.pools,
    poolSizes: s.poolSizes,
    tiebreaks: s.tiebreaks,
    crossPool: s.crossPool,
    fullPlacement: s.fullPlacement,
    routing: s.routing,
  };
}

function places(from: number, to: number): string {
  return from === to ? `${from}` : `${from}–${to}`;
}

/** An edge that leaves a stage downwards, runs along the lane at y and rises into the output, labelled near its start. */
function laneEdge(a: { x: number; y: number }, b: { x: number; y: number }, y: number, label: string): GraphEdge {
  const d = [
    `M ${a.x} ${a.y}`,
    `C ${a.x + 40} ${a.y}, ${a.x + 40} ${y}, ${a.x + 80} ${y}`,
    `L ${b.x - 80} ${y}`,
    `C ${b.x - 40} ${y}, ${b.x - 40} ${b.y}, ${b.x} ${b.y}`,
  ].join(' ');
  return { d, label, x: a.x + COL_GAP / 2 + 40, y };
}

/** A curved edge between two points, with its label in the middle. */
function edge(a: { x: number; y: number }, b: { x: number; y: number }, label: string): GraphEdge {
  const mid = (a.x + b.x) / 2;
  return { d: `M ${a.x} ${a.y} C ${mid} ${a.y}, ${mid} ${b.y}, ${b.x} ${b.y}`, label, x: mid, y: (a.y + b.y) / 2 };
}
