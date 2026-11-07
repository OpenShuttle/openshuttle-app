import { TranslocoService } from '@jsverse/transloco';
import {
  ControlDeskView,
  MatchSource,
  MatchView,
  PhasesView,
  PoolPhase,
  RoundView,
  ScheduleSlotView,
  ScheduleView,
  SideView,
  StagePhase,
  TournamentView,
} from '@bindings/apps/desktop';
import { StageKind } from '@bindings/internal/tournament';

import { bracketMatchName, sourceLabel } from '../shared/bracket-labels';
import { roundLabel, shortRoundLabel } from '../shared/match-stage';
import { clubs, fullNames } from '../shared/names';
import { stageLabel } from '../shared/stage-label';

/** The views a snapshot is built from, all taken at one tournament version. */
export interface SnapshotData {
  tournament: TournamentView;
  phases: PhasesView;
  desk: ControlDeskView;
  schedule: ScheduleView;
}

export interface SnapshotOptions {
  /** "14:05" in the UI language. */
  time: (value: string | Date) => string;
  /** "19 octobre 2026". */
  date: (value: string | Date) => string;
  showBibs: boolean;
  lang: string;
  /** When the snapshot is built. */
  now: Date;
}

/**
 * The readable state of a tournament, as one self-contained HTML page: what
 * the courts play, pending scores, every stage (pool standings and matches,
 * bracket rounds and places) and the matches left in the schedule order. It
 * is written beside the tournament file after each action, for the desk to
 * go on by hand if the app or the computer fails.
 */
export function snapshotHtml(t: TranslocoService, data: SnapshotData, o: SnapshotOptions): string {
  const tr = (key: string, params?: Record<string, unknown>) => t.translate(key, params);
  const team = (side: SideView) => (o.showBibs && side.bib ? `#${side.bib} ${fullNames(side)}` : fullNames(side));
  const short = (m: { category: string }) => tr('category.short.' + m.category);

  /** "Victor Gauthier / David Michel (AS Bron)", or where the side comes from. */
  const sideCell = (side: SideView, bye: boolean, from: MatchSource | null | undefined): string => {
    if (bye) {
      return `<span class="muted">${esc(tr('phases.bye'))}</span>`;
    }
    if (!side.entrant) {
      return `<span class="muted">${esc(from ? sourceLabel(t, from) : tr('phases.tbd'))}</span>`;
    }
    const c = clubs(side);
    const name = esc(team(side));
    const shown = side.withdrawn ? `<s>${name}</s>` : name;
    return c ? `${shown} <span class="muted">(${esc(c)})</span>` : shown;
  };

  /** The score, the forfeit, or where the match stands. */
  const result = (m: MatchView): string => {
    if (m.walkoverA || m.walkoverB || m.walkover) {
      const who = [m.walkoverA ? team(m.a) : '', m.walkoverB ? team(m.b) : ''].filter((w) => w).join(', ');
      return esc(who ? tr('desk.walkoverBib', { who }) : tr('desk.walkover'));
    }
    if (m.score?.sets?.length) {
      const sets = m.score.sets.map((s) => `${s.a}-${s.b}`).join('  ');
      return `<strong>${esc(sets)}</strong>${m.score.goldenPoint ? ` <span class="muted">${esc(tr('desk.goldenPoint'))}</span>` : ''}`;
    }
    if (m.calledAt && !m.endedAt) {
      return `<span class="tag live">${esc(tr('phases.onCourt', { n: m.court }))}</span>`;
    }
    if (m.endedAt) {
      return `<span class="tag pending">${esc(tr('phases.scorePending'))}</span>`;
    }
    return `<span class="muted">${esc(tr('phases.toPlay'))}</span>`;
  };

  /** "Pool A · round 2", "Main bracket · QF A". */
  const where = (m: MatchView): string => {
    const stage = stageLabel(t, { name: m.stageName, kind: m.stageKind });
    if (m.stageKind === StageKind.StageBracket) {
      return `${stage} · ${bracketMatchName(t, m)}`;
    }
    const pool = tr('desk.pool', { pool: m.pool });
    return `${m.entryStage ? pool : `${stage} · ${pool}`} · ${tr('schedule.round', { n: m.round })}`;
  };

  const matchRow = (m: MatchView, first: string): string =>
    `<tr><td class="nowrap">${first}</td><td>${sideCell(m.a, m.byeA, m.fromA)}</td><td>${sideCell(m.b, m.byeB, m.fromB)}</td><td class="nowrap">${result(m)}</td></tr>`;

  const parts: string[] = [];
  const v = data.tournament;
  parts.push(`<header><h1>${esc(v.name)}</h1>`);
  parts.push(`<p class="stamp">${esc(tr('snapshot.updatedAt', { date: o.date(o.now), time: o.time(o.now) }))}</p>`);
  parts.push(`<p class="note">${esc(tr('snapshot.note', { file: baseName(v.path) }))}</p></header>`);

  // Courts and pending scores.
  if (data.desk.started) {
    const rows = (data.desk.courts ?? []).map((c) => {
      const m = c.match;
      const state = tr('desk.state.' + c.state);
      if (!m) {
        return `<tr><td class="nowrap">${esc(tr('desk.courtN', { n: c.number }))}</td><td class="muted">${esc(state)}</td><td colspan="3"></td></tr>`;
      }
      // Since when: the call, or the end once the desk ended it.
      const since = o.time((c.state === 'finished' ? m.endedAt : null) ?? m.calledAt ?? '');
      return `<tr><td class="nowrap">${esc(tr('desk.courtN', { n: c.number }))}</td><td class="nowrap">${esc(state)} ${esc(since)}</td><td class="nowrap"><span class="badge">${esc(short(m))}</span> ${esc(where(m))}</td><td>${sideCell(m.a, false, null)}</td><td>${sideCell(m.b, false, null)}</td></tr>`;
    });
    parts.push(section(tr('snapshot.courts'), table(rows)));

    const pending = (data.desk.pending ?? []).map((m) =>
      matchRow(m, `<span class="badge">${esc(short(m))}</span> ${esc(where(m))}<br><span class="muted">${esc(tr('desk.endedAt', { time: o.time(m.endedAt ?? '') }))}</span>`),
    );
    parts.push(section(tr('desk.pendingScores'), pending.length ? table(pending) : `<p class="muted">${esc(tr('desk.noPending'))}</p>`));
  }

  // Every stage, category by category.
  for (const c of data.phases.categories ?? []) {
    const stages: string[] = [];
    if (!c.started) {
      stages.push(`<p class="muted">${esc(tr('snapshot.notDrawn'))}</p>`);
    }
    for (const s of c.started ? (c.stages ?? []) : []) {
      stages.push(stageHtml(s));
    }
    parts.push(`<section class="category"><h2><span class="badge">${esc(tr('category.short.' + c.category))}</span> ${esc(tr('category.name.' + c.category))}</h2>${stages.join('')}</section>`);
  }

  // Matches left, in the schedule order.
  const left = (data.schedule.slots ?? []).filter((s) => s.state === 'planned');
  if (data.desk.started || left.length) {
    const rows = left.map((s) => leftRow(s));
    parts.push(
      section(
        tr('snapshot.left', { n: left.length }),
        rows.length
          ? table(rows, [tr('snapshot.estimated'), tr('desk.court'), tr('snapshot.match'), '', ''])
          : `<p class="muted">${esc(tr('schedule.nothingLeft'))}</p>`,
      ),
    );
  }

  return `<!doctype html>
<html lang="${esc(o.lang)}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="generator" content="OpenShuttle">
<title>${esc(v.name)}</title>
<style>${STYLE}</style>
</head>
<body>
${parts.join('\n')}
</body>
</html>
`;

  function stageHtml(s: StagePhase): string {
    const title = esc(stageLabel(t, { name: s.name, kind: s.kind }));
    if (!s.confirmed) {
      return `<h3>${title}</h3><p class="muted">${esc(tr('snapshot.notFilled'))}</p>`;
    }
    const body = s.kind === StageKind.StagePools ? (s.pools ?? []).map(poolHtml).join('') : bracketHtml(s);
    const status = s.complete ? ` <span class="tag done">${esc(tr('phases.bracketStatus.finished'))}</span>` : '';
    return `<h3>${title}${status}</h3>${body}`;
  }

  function poolHtml(p: PoolPhase): string {
    const head = [tr('phases.rankShort'), tr('phases.entrant'), tr('phases.played'), tr('phases.wins'), tr('phases.sets'), tr('phases.pointsShort'), ''];
    const rows = (p.standings ?? []).map((r) => {
      const flags = [r.withdrawn ? tr('phases.forfeit') : '', r.drawNeeded ? tr('phases.tied') : ''].filter((f) => f).join(' · ');
      return `<tr><td class="num">${r.rank || '–'}</td><td>${sideCell(r.entrant, false, null)}</td><td class="num">${r.played}</td><td class="num">${r.wins}</td><td class="num">${diff(r.setsWon - r.setsLost)}</td><td class="num">${diff(r.pointsWon - r.pointsLost)}</td><td>${esc(flags)}</td></tr>`;
    });
    const matches = (p.matches ?? []).map((m) => matchRow(m, esc(tr('schedule.round', { n: m.round }))));
    const played = (p.matches ?? []).filter((m) => m.score || m.walkover || m.walkoverA || m.walkoverB).length;
    return `<div class="pool"><h4>${esc(tr('desk.pool', { pool: p.name }))} <span class="muted">${esc(tr('phases.playedOf', { n: played, total: (p.matches ?? []).length }))}</span></h4>${table(rows, head)}${table(matches)}</div>`;
  }

  function bracketHtml(s: StagePhase): string {
    const out: string[] = [];
    for (const r of s.rounds ?? []) {
      out.push(`<h4>${esc(roundTitle(r))}</h4>`);
      out.push(table((r.matches ?? []).map((m) => matchRow(m, esc(bracketMatchName(t, m))))));
    }
    const places = (s.places ?? []).filter((p) => p.entrant.entrant);
    if (places.length) {
      const rows = places.map((p) => `<tr><td class="num">${p.from === p.to ? p.from : `${p.from}–${p.to}`}</td><td>${sideCell(p.entrant, false, null)}</td></tr>`);
      out.push(`<h4>${esc(tr('phases.places'))}</h4>${table(rows)}`);
    }
    return out.join('');
  }

  function roundTitle(r: RoundView): string {
    const l = roundLabel({ placeFrom: r.from, placeTo: r.to, roundSize: r.size });
    return tr(l.key, l.params);
  }

  function leftRow(s: ScheduleSlotView): string {
    const stage = s.acronym || tr('stage.acronym.' + s.stageKind);
    const r = s.match ? shortRoundLabel(s.match) : shortRoundLabel({ placeFrom: s.placeFrom || 1, placeTo: s.placeTo, roundSize: s.roundSize });
    const label = s.pool
      ? tr('schedule.poolRound', { stage, name: s.pool, round: s.round, rounds: s.rounds || s.round })
      : tr('schedule.bracketRound', { stage, round: tr(r.key, r.params) });
    const sides = s.match
      ? `<td>${sideCell(s.match.a, s.match.byeA, s.match.fromA)}</td><td>${sideCell(s.match.b, s.match.byeB, s.match.fromB)}</td>`
      : `<td colspan="2" class="muted">${esc(tr('schedule.unknownPlayers'))}</td>`;
    return `<tr><td class="nowrap">${esc(o.time(s.start))}</td><td class="num">${s.court || ''}</td><td class="nowrap"><span class="badge">${esc(short(s))}</span> ${esc(label)}</td>${sides}</tr>`;
  }
}

function section(title: string, body: string): string {
  return `<section><h2>${esc(title)}</h2>${body}</section>`;
}

function table(rows: string[], head?: string[]): string {
  if (!rows.length) {
    return '';
  }
  const thead = head ? `<thead><tr>${head.map((h) => `<th>${esc(h)}</th>`).join('')}</tr></thead>` : '';
  return `<table>${thead}<tbody>${rows.join('')}</tbody></table>`;
}

function diff(n: number): string {
  return n > 0 ? `+${n}` : String(n);
}

function baseName(path: string): string {
  return path.split(/[\\/]/).pop() ?? path;
}

/** Escapes text for HTML content and attribute values. */
export function esc(text: string | number): string {
  return String(text).replace(/[&<>"']/g, (c) => ESCAPES[c]);
}

const ESCAPES: Record<string, string> = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' };

const STYLE = `
body{font:14px/1.4 system-ui,-apple-system,"Segoe UI",Roboto,sans-serif;color:#1d2433;background:#fff;margin:0 auto;padding:16px;max-width:1100px}
h1{font-size:22px;margin:0}
h2{font-size:18px;margin:28px 0 8px;border-bottom:2px solid #1d2433;padding-bottom:4px}
h3{font-size:16px;margin:18px 0 6px}
h4{font-size:14px;margin:12px 0 4px}
.stamp{margin:4px 0;font-weight:600}
.note{margin:4px 0;color:#5b6475;font-size:13px}
.muted{color:#6b7280}
table{border-collapse:collapse;width:100%;margin:4px 0 8px}
th,td{border:1px solid #d6dae1;padding:3px 6px;text-align:left;vertical-align:top}
th{background:#f2f4f7;font-weight:600}
.num{text-align:right;white-space:nowrap}
.nowrap{white-space:nowrap}
.badge{display:inline-block;border:1px solid #1d2433;border-radius:4px;padding:0 4px;font-size:12px;font-weight:700}
.tag{border-radius:4px;padding:0 4px;font-size:12px}
.live{background:#dcfce7}
.pending{background:#fef3c7}
.done{background:#e5e7eb;font-weight:400}
.pool{break-inside:avoid}
@media print{body{padding:0;font-size:12px}h2{break-after:avoid}}
`;
