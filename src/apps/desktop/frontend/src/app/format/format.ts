import { Component, computed, inject, signal } from '@angular/core';
import { RouterLink } from '@angular/router';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { FormatView, PresetService, StageView, TournamentService } from '@bindings/apps/desktop';
import { Category, Tiebreak, WinCondition, WinConditionKind } from '@bindings/internal/engine';
import { StageConfig, StageKind } from '@bindings/internal/tournament';

import { liveView } from '../core/live-view';
import { AppPresets } from '../core/presets.service';
import { TournamentStore } from '../core/tournament.service';
import { stageLabel, winSummary } from '../shared/stage-label';

const MAX_POOLS = 16;
const DEFAULT_SETS: WinCondition = { kind: WinConditionKind.WinBySets, setsToWin: 2, pointsPerSet: 21, cap: 30 };
const DEFAULT_TIMED: WinCondition = { kind: WinConditionKind.WinByTime, minutes: 12 };
const POINT_CHOICES = [11, 15, 21];

/** Canvas geometry of the stage graph, in pixels. */
const NODE_W = 190;
const NODE_H = 140;
const COL_GAP = 84;
const ROW_GAP = 26;
const PAD = 16;
const OUTPUT_W = 170;

/** Selects the last stage, whatever its id: the one just added. */
const LAST = '__last';

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

/** What the save-as-preset form saves. */
type PresetKind = 'format' | 'win' | 'tiebreaks';

/**
 * Format: the stages of each category as a graph (phase by phase, linked by
 * routing), the selected stage edited in the side panel. Every change sends
 * the whole format to Go, which validates it.
 */
@Component({
  selector: 'app-format',
  imports: [TranslocoDirective, RouterLink],
  templateUrl: './format.html',
  styleUrl: './format.scss',
})
export class Format {
  protected readonly tournament = inject(TournamentStore);
  protected readonly presets = inject(AppPresets);
  private readonly transloco = inject(TranslocoService);
  protected readonly view = liveView<FormatView>(() => TournamentService.GetFormatView(), { categories: [] });
  protected readonly maxPools = MAX_POOLS;
  protected readonly kinds = WinConditionKind;
  protected readonly stageKinds = StageKind;
  protected readonly pointChoices = POINT_CHOICES;
  protected readonly nodeW = NODE_W;
  protected readonly nodeH = NODE_H;
  protected readonly outputW = OUTPUT_W;

  private readonly selectedCategory = signal<Category | null>(null);
  protected readonly current = computed(() => {
    const categories = this.view().categories ?? [];
    return categories.find((c) => c.category === this.selectedCategory()) ?? categories[0] ?? null;
  });
  protected readonly stages = computed(() => this.current()?.stages ?? []);

  /** Selected stage ID; the entry stage by default, the last one after an add. */
  private readonly selectedID = signal('');
  protected readonly selected = computed(() => {
    const stages = this.stages();
    const id = this.selectedID();
    return (id === LAST ? stages[stages.length - 1] : stages.find((s) => s.id === id)) ?? stages[0] ?? null;
  });

  /** The save-as-preset form, when open. */
  protected readonly saving = signal<PresetKind | null>(null);
  protected readonly presetName = signal('');
  protected readonly presetDescription = signal('');

  // Graph layout: one column per phase, the output last.
  private readonly phases = computed(() => Math.max(1, ...this.stages().map((s) => s.phase)));
  protected readonly canvasHeight = computed(() => {
    const perPhase = new Map<number, number>();
    for (const s of this.stages()) {
      perPhase.set(s.phase, (perPhase.get(s.phase) ?? 0) + 1);
    }
    const rows = Math.max(1, ...perPhase.values());
    return Math.max(220, rows * (NODE_H + ROW_GAP) - ROW_GAP + 2 * PAD);
  });
  protected readonly canvasWidth = computed(() => PAD + this.phases() * (NODE_W + COL_GAP) + OUTPUT_W + PAD);
  protected readonly outputX = computed(() => PAD + this.phases() * (NODE_W + COL_GAP));
  protected readonly outputY = computed(() => this.canvasHeight() / 2 - NODE_H / 2);

  protected readonly nodes = computed<GraphNode[]>(() => {
    const h = this.canvasHeight();
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
      for (const target of new Set((n.stage.routing ?? []).filter((r) => r))) {
        const to = nodes.get(target);
        if (to) {
          const label = this.transloco.translate('format.ranks', { ranks: this.ranksTo(n.stage, target) });
          out.push(edge(from, { x: to.x, y: to.y + NODE_H / 2 }, label));
        }
      }
      if (n.stage.placeFrom > 0) {
        out.push(edge(from, exit, places(n.stage.placeFrom, n.stage.placeTo)));
      }
    }
    return out;
  });

  protected readonly totalMatches = computed(() => this.stages().reduce((n, s) => n + s.matches, 0));

  protected selectCategory(category: Category): void {
    this.selectedCategory.set(category);
    this.selectedID.set('');
  }

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

  /** "1–2" for the ranks a stage routes to another, "1, 3" otherwise. */
  protected ranksTo(stage: StageView, target: string): string {
    const ranks = (stage.routing ?? []).map((r, i) => (r === target ? i + 1 : 0)).filter((r) => r > 0);
    const contiguous = ranks.every((r, i) => i === 0 || r === ranks[i - 1] + 1);
    return contiguous && ranks.length > 1 ? `${ranks[0]}–${ranks[ranks.length - 1]}` : ranks.join(', ');
  }

  /** "3 pools of 4" or "3 pools (4, 4, 3)". */
  protected poolsTitle(stage: StageView): string {
    const sizes = stage.sizes ?? [];
    if (sizes.length === 0 || sizes.every((s) => s === 0)) {
      return this.transloco.translate('format.poolsN', { n: stage.pools });
    }
    return sizes.every((s) => s === sizes[0])
      ? this.transloco.translate('format.poolsOf', { n: stage.pools, size: sizes[0] })
      : this.transloco.translate('format.poolsSizes', { n: stage.pools, sizes: sizes.join(', ') });
  }

  /** Stages after this one: where its ranks can go. */
  protected laterStages(stage: StageView): StageView[] {
    const stages = this.stages();
    return stages.slice(stages.findIndex((s) => s.id === stage.id) + 1);
  }

  // Edits: each sends the whole format.

  protected edit(stage: StageView, change: Partial<StageConfig>): void {
    const fc = this.current();
    if (!fc) {
      return;
    }
    const configs = this.stages().map((s) => (s.id === stage.id ? { ...config(s), ...change } : config(s)));
    void this.tournament.setFormat(fc.category, configs);
  }

  protected setKind(stage: StageView, kind: StageKind): void {
    if (kind === stage.kind) {
      return;
    }
    this.edit(
      stage,
      kind === StageKind.StagePools
        ? { kind, pools: 2, tiebreaks: [...(this.presets.value().tiebreaks?.[0]?.tiebreaks ?? [])], fullPlacement: false }
        : { kind, pools: 0, tiebreaks: [], fullPlacement: true },
    );
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

  protected setRoute(stage: StageView, rank: number, event: Event): void {
    const routing = [...(stage.routing ?? [])];
    while (routing.length < rank) {
      routing.push('');
    }
    routing[rank - 1] = (event.target as HTMLSelectElement).value;
    this.edit(stage, { routing });
  }

  protected routeOf(stage: StageView, rank: number): string {
    return stage.routing?.[rank - 1] ?? '';
  }

  protected async addStage(kind: StageKind): Promise<void> {
    const fc = this.current();
    if (fc && (await this.tournament.addStage(fc.category, kind))) {
      this.selectedID.set(LAST);
    }
  }

  protected removeStage(stage: StageView): void {
    const fc = this.current();
    if (fc) {
      void this.tournament.removeStage(fc.category, stage.id);
      this.selectedID.set('');
    }
  }

  protected applyFormatPreset(event: Event): void {
    const fc = this.current();
    const select = event.target as HTMLSelectElement;
    const id = select.value;
    select.value = '';
    if (fc && id) {
      void this.tournament.applyFormatPreset(fc.category, id);
      this.selectedID.set('');
    }
  }

  protected formatPresetLabel(p: { key: string; name: string }): string {
    return p.name || this.transloco.translate('presets.format.' + p.key + '.name');
  }

  protected tiebreakPresetLabel(p: { key: string; name: string }): string {
    return p.name || this.transloco.translate('presets.tiebreaks.' + p.key);
  }

  protected value(event: Event): string {
    return (event.target as HTMLInputElement).value;
  }

  protected openSave(kind: PresetKind): void {
    this.presetName.set('');
    this.presetDescription.set('');
    this.saving.set(kind);
  }

  protected async savePreset(): Promise<void> {
    const fc = this.current();
    const stage = this.selected();
    const name = this.presetName();
    let done = false;
    switch (this.saving()) {
      case 'format':
        done = !!fc && (await this.tournament.saveFormatPreset(fc.category, name, this.presetDescription()));
        break;
      case 'win':
        done = !!stage && (await this.presets.run(() => PresetService.SaveWinPreset(name, stage.win)));
        break;
      case 'tiebreaks':
        done = !!stage && (await this.presets.run(() => PresetService.SaveTiebreakPreset(name, stage.tiebreaks ?? [])));
        break;
    }
    if (done) {
      this.saving.set(null);
    }
  }

  protected tiebreaks(stage: StageView): Tiebreak[] {
    return stage.tiebreaks ?? [];
  }
}

function config(s: StageView): StageConfig {
  return {
    id: s.id,
    name: s.name,
    kind: s.kind,
    win: s.win,
    pools: s.pools,
    tiebreaks: s.tiebreaks,
    fullPlacement: s.fullPlacement,
    routing: s.routing,
  };
}

function places(from: number, to: number): string {
  return from === to ? `${from}` : `${from}–${to}`;
}

/** A curved edge between two points, with its label in the middle. */
function edge(a: { x: number; y: number }, b: { x: number; y: number }, label: string): GraphEdge {
  const mid = (a.x + b.x) / 2;
  return { d: `M ${a.x} ${a.y} C ${mid} ${a.y}, ${mid} ${b.y}, ${b.x} ${b.y}`, label, x: mid, y: (a.y + b.y) / 2 };
}
