import { Component, computed, inject, signal } from '@angular/core';
import { RouterLink } from '@angular/router';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { BracketView, FormatCategory, FormatView, TournamentService } from '@bindings/apps/desktop';
import { Category, Tiebreak, WinCondition, WinConditionKind } from '@bindings/internal/engine';
import { BracketConfig, PoolStage } from '@bindings/internal/tournament';

import { liveView } from '../core/live-view';
import { TournamentStore } from '../core/tournament.service';

const MAX_POOLS = 16;
const DEFAULT_SETS: WinCondition = { kind: WinConditionKind.WinBySets, setsToWin: 2, pointsPerSet: 21, cap: 30 };
const DEFAULT_TIMED: WinCondition = { kind: WinConditionKind.WinByTime, minutes: 12 };
const POINT_CHOICES = [11, 15, 21];

/** Canvas geometry of the stage graph, in pixels. */
const NODE_W = 190;
const NODE_H = 140;
const COL_POOLS = 16;
const COL_BRACKETS = 262;
const COL_OUTPUT = 508;
const OUTPUT_W = 170;
/** Canvas width: fits the narrowest window. */
const CANVAS_W = COL_OUTPUT + OUTPUT_W + 16;
const ROW_GAP = 30;
/** Selects the last bracket, whatever its id. */
const LAST = '__last';

interface GraphNode {
  /** 'pools' or a bracket id. */
  key: string;
  x: number;
  y: number;
  bracket: BracketView | null;
}

interface GraphEdge {
  d: string;
  label: string;
  x: number;
  y: number;
}

/** Format: the stages of each category as a graph, edited in the side panel. */
@Component({
  selector: 'app-format',
  imports: [TranslocoDirective, RouterLink],
  templateUrl: './format.html',
  styleUrl: './format.scss',
})
export class Format {
  protected readonly tournament = inject(TournamentStore);
  private readonly transloco = inject(TranslocoService);
  protected readonly view = liveView<FormatView>(() => TournamentService.GetFormatView(), {
    categories: [],
  });
  protected readonly maxPools = MAX_POOLS;
  protected readonly kinds = WinConditionKind;
  protected readonly pointChoices = POINT_CHOICES;
  protected readonly nodeW = NODE_W;
  protected readonly nodeH = NODE_H;
  protected readonly outputX = COL_OUTPUT;
  protected readonly outputW = OUTPUT_W;
  protected readonly canvasW = CANVAS_W;

  private readonly selectedCategory = signal<Category | null>(null);
  protected readonly current = computed(() => {
    const categories = this.view().categories ?? [];
    return categories.find((c) => c.category === this.selectedCategory()) ?? categories[0] ?? null;
  });

  /** Selected node: 'pools' or a bracket id. */
  private readonly selectedNode = signal('pools');
  protected readonly selectedBracket = computed(() => {
    const id = this.selectedNode();
    const brackets = this.current()?.brackets ?? [];
    return id === LAST ? (brackets[brackets.length - 1] ?? null) : (brackets.find((b) => b.id === id) ?? null);
  });

  protected readonly canvasHeight = computed(() => {
    const n = Math.max(1, (this.current()?.brackets ?? []).length);
    return Math.max(260, n * (NODE_H + ROW_GAP) + ROW_GAP);
  });

  protected readonly nodes = computed<GraphNode[]>(() => {
    const fc = this.current();
    const h = this.canvasHeight();
    const middle = h / 2 - NODE_H / 2;
    const brackets = fc?.brackets ?? [];
    const top = h / 2 - (brackets.length * (NODE_H + ROW_GAP) - ROW_GAP) / 2;
    return [
      { key: 'pools', x: COL_POOLS, y: middle, bracket: null },
      ...brackets.map((b, i) => ({ key: b.id, x: COL_BRACKETS, y: top + i * (NODE_H + ROW_GAP), bracket: b })),
    ];
  });

  protected readonly outputY = computed(() => this.canvasHeight() / 2 - NODE_H / 2);

  protected readonly edges = computed<GraphEdge[]>(() => {
    const fc = this.current();
    if (!fc) {
      return [];
    }
    const [pools, ...brackets] = this.nodes();
    const from = { x: pools.x + NODE_W, y: pools.y + NODE_H / 2 };
    const out = { x: COL_OUTPUT, y: this.outputY() + NODE_H / 2 };
    const edges: GraphEdge[] = [];
    for (const node of brackets) {
      const b = node.bracket!;
      const ranks = this.ranksTo(fc, b.id);
      const to = { x: node.x, y: node.y + NODE_H / 2 };
      if (ranks) {
        edges.push(edge(from, to, this.transloco.translate('format.ranks', { ranks })));
      }
      if (b.entrants > 0) {
        edges.push(edge({ x: node.x + NODE_W, y: to.y }, out, `${b.placeFrom}–${b.placeTo}`));
      }
    }
    if (brackets.length === 0) {
      edges.push(edge(from, out, ''));
    }
    return edges;
  });

  /** Total matches of the category, pools and brackets. */
  protected readonly totalMatches = computed(() => {
    const fc = this.current();
    return fc ? fc.matches + (fc.brackets ?? []).reduce((n, b) => n + b.matches, 0) : 0;
  });

  /** True when every entrant gets a place of its own. */
  protected readonly noTies = computed(() => {
    const fc = this.current();
    if (!fc || (fc.brackets ?? []).length === 0) {
      return false;
    }
    const routed = (fc.brackets ?? []).reduce((n, b) => n + b.entrants, 0);
    return routed === fc.registered && (fc.brackets ?? []).every((b) => b.fullPlacement || b.entrants === 0);
  });

  protected selectCategory(category: Category): void {
    this.selectedCategory.set(category);
    this.selectedNode.set('pools');
  }

  protected selectNode(key: string): void {
    this.selectedNode.set(key);
  }

  protected isSelected(key: string): boolean {
    return key === 'pools' ? this.selectedBracket() === null : this.selectedBracket()?.id === key;
  }

  /** "1–2" for the pool ranks routed to a bracket, "" if none. */
  protected ranksTo(fc: FormatCategory, id: string): string {
    const ranks = (fc.routing ?? []).map((r, i) => (r === id ? i + 1 : 0)).filter((r) => r > 0);
    if (ranks.length === 0) {
      return '';
    }
    const contiguous = ranks.every((r, i) => i === 0 || r === ranks[i - 1] + 1);
    return contiguous && ranks.length > 1 ? `${ranks[0]}–${ranks[ranks.length - 1]}` : ranks.join(', ');
  }

  /** "3 pools of 4" or "3 pools (4, 4, 3)". */
  protected poolsTitle(fc: FormatCategory): string {
    const sizes = fc.sizes ?? [];
    const pools = fc.stage.pools;
    if (sizes.length === 0) {
      return this.transloco.translate('format.poolsN', { n: pools });
    }
    return sizes.every((s) => s === sizes[0])
      ? this.transloco.translate('format.poolsOf', { n: pools, size: sizes[0] })
      : this.transloco.translate('format.poolsSizes', { n: pools, sizes: sizes.join(', ') });
  }

  protected winSummary(win: WinCondition): string {
    if (win.kind === WinConditionKind.WinByTime) {
      return this.transloco.translate('format.timedSummary', { minutes: win.minutes ?? 0 });
    }
    return this.transloco.translate('format.setsSummary', {
      sets: win.setsToWin ?? 1,
      points: win.pointsPerSet ?? 21,
      cap: win.cap ?? 0,
    });
  }

  protected setPools(fc: FormatCategory, pools: number): void {
    if (pools >= 1 && pools <= MAX_POOLS) {
      this.save(fc, { pools });
    }
  }

  /** Win condition of the selected stage. */
  protected winOf(fc: FormatCategory): WinCondition {
    return this.selectedBracket()?.win ?? fc.stage.win;
  }

  protected setKind(fc: FormatCategory, kind: WinConditionKind): void {
    if (kind !== this.winOf(fc).kind) {
      this.saveWin(fc, kind === WinConditionKind.WinByTime ? DEFAULT_TIMED : DEFAULT_SETS);
    }
  }

  protected stepWin(fc: FormatCategory, field: 'setsToWin' | 'cap' | 'minutes', delta: number): void {
    const win = this.winOf(fc);
    this.saveWin(fc, { ...win, [field]: (win[field] ?? 0) + delta });
  }

  protected setPoints(fc: FormatCategory, points: number): void {
    this.saveWin(fc, { ...this.winOf(fc), pointsPerSet: points });
  }

  private saveWin(fc: FormatCategory, win: WinCondition): void {
    const b = this.selectedBracket();
    if (b) {
      this.setBracket(fc, b, { win });
    } else {
      this.save(fc, { win });
    }
  }

  /** Moves a tiebreak one step up (-1) or down (+1). */
  protected moveTiebreak(fc: FormatCategory, index: number, step: -1 | 1): void {
    const tiebreaks = [...(fc.stage.tiebreaks ?? [])];
    const target = index + step;
    if (target < 0 || target >= tiebreaks.length) {
      return;
    }
    [tiebreaks[index], tiebreaks[target]] = [tiebreaks[target], tiebreaks[index]];
    this.save(fc, { tiebreaks });
  }

  protected tiebreaks(fc: FormatCategory): Tiebreak[] {
    return fc.stage.tiebreaks ?? [];
  }

  protected async addBracket(fc: FormatCategory): Promise<void> {
    const n = (fc.brackets ?? []).length;
    const name = this.transloco.translate(n === 0 ? 'format.mainBracket' : 'format.bracketN', { n: n + 1 });
    if (await this.tournament.addBracket(fc.category, { name, win: { ...DEFAULT_SETS }, fullPlacement: true })) {
      this.selectedNode.set(LAST); // the new bracket, once refetched
    }
  }

  protected removeBracket(fc: FormatCategory, b: BracketView): void {
    void this.tournament.removeBracket(fc.category, b.id);
    this.selectedNode.set('pools');
  }

  protected setBracket(fc: FormatCategory, b: BracketView, change: Partial<BracketConfig>): void {
    const cfg: BracketConfig = { name: b.name, win: b.win, fullPlacement: b.fullPlacement };
    void this.tournament.configureBracket(fc.category, b.id, { ...cfg, ...change });
  }

  protected value(event: Event): string {
    return (event.target as HTMLInputElement).value;
  }

  /** Pool positions 1..rows with their destination ("" for eliminated). */
  protected routes(fc: FormatCategory): { position: number; target: string; count: number }[] {
    const sizes = fc.sizes ?? [];
    return Array.from({ length: fc.routingRows }, (_, i) => ({
      position: i + 1,
      target: fc.routing?.[i] ?? '',
      count: sizes.filter((s) => s > i).length,
    }));
  }

  protected setRoute(fc: FormatCategory, position: number, event: Event): void {
    const routing = this.routes(fc).map((r) => r.target);
    routing[position - 1] = (event.target as HTMLSelectElement).value;
    void this.tournament.setRouting(fc.category, routing);
  }

  private save(fc: FormatCategory, change: Partial<PoolStage>): void {
    void this.tournament.configurePoolStage(fc.category, { ...fc.stage, ...change });
  }
}

/** A curved edge between two points, with its label in the middle. */
function edge(a: { x: number; y: number }, b: { x: number; y: number }, label: string): GraphEdge {
  const mid = (a.x + b.x) / 2;
  return {
    d: `M ${a.x} ${a.y} C ${mid} ${a.y}, ${mid} ${b.y}, ${b.x} ${b.y}`,
    label,
    x: mid,
    y: (a.y + b.y) / 2,
  };
}
