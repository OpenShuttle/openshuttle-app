import { ScheduleView, TournamentView } from '@bindings/apps/desktop';

import { FakeBackend } from '../../testing/backend';
import { choose, click, clickAll, drag, find, render, text, type } from '../../testing/render';
import before from '../../testing/fixtures/before-schedule.json';
import schedule from '../../testing/fixtures/progress-schedule.json';
import rules from '../../testing/fixtures/progress-schedule-rules.json';
import validated from '../../testing/fixtures/progress-schedule-validated.json';
import tournament from '../../testing/fixtures/progress-tournament.json';
import { Schedule } from './schedule';

describe('Schedule', () => {
  let go: FakeBackend;
  const view = (v: unknown) => go.on('TournamentService.GetScheduleView', v as ScheduleView);

  beforeEach(() => {
    localStorage.clear();
    go = new FakeBackend().on('TournamentService.GetTournamentView', tournament as unknown as TournamentView);
    view(schedule);
  });

  it('lists the rounds and draws the courts', async () => {
    const f = await render(Schedule);
    expect(text(f)).toContain('Order of rounds');
    expect(f.nativeElement.querySelectorAll('.block-row').length).toBe(schedule.blocks.length);
    expect(f.nativeElement.querySelectorAll('.slot').length).toBe(schedule.slots.length);
  });

  it('names rounds by acronym, k/n and short bracket rounds, colored by pool or bracket', async () => {
    const f = await render(Schedule);
    const labels = [...f.nativeElement.querySelectorAll('.block-label')].map((e) => (e as HTMLElement).textContent?.trim());
    expect(labels).toContain('P A · 2/3');
    expect(labels).toContain('B · QF');
    expect(labels).toContain('B · 5th–8th');
    expect(labels).toContain('B · 3rd place');
    expect(labels).toContain('B · Final');
    const badge = f.nativeElement.querySelector('.block-row .badge') as HTMLElement;
    expect(badge.getAttribute('style')).toContain('--color-group-4');
  });

  it('shows the acronym typed in the format', async () => {
    view({ ...schedule, blocks: schedule.blocks.map((b) => ({ ...b, acronym: b.stageKind === 'bracket' ? 'T1' : 'Q' })) });
    const f = await render(Schedule);
    expect(text(f)).toContain('T1 · QF');
    expect(text(f)).toContain('Q A · 2/3');
  });

  it('adds, draws and removes breaks; shows the start and a late end', async () => {
    const at = (min: number) => new Date(Date.parse(schedule.slots[0].start) + min * 60_000).toISOString();
    view({ ...schedule, start: at(0), dayEnd: at(30), late: true, breaks: [{ id: 'k9', from: at(20), to: at(50) }] });
    const f = await render(Schedule);
    expect(find(f, '[data-start]').textContent).toContain('Starts');
    expect(find(f, '[data-late]').textContent).toContain('Ends after');
    expect(f.nativeElement.querySelectorAll('.break-band').length).toBe(schedule.courts);
    expect(f.nativeElement.querySelector('.now')).toBeNull();
    const [from, to] = [...f.nativeElement.querySelectorAll('.break-form input')] as HTMLInputElement[];
    await type(f, from, '12:00');
    await type(f, to, '13:30');
    await click(f, 'Add a break');
    expect(go.argsOf('TournamentService.AddBreak')).toEqual([['12:00', '13:30']]);
    (find(f, '.break-remove') as HTMLButtonElement).click();
    expect(go.argsOf('TournamentService.RemoveBreak')).toEqual([['k9']]);
  });

  it('adds a ghost match after a match of its court, draws it and removes it', async () => {
    const anchor = schedule.slots.find((s) => s.state === 'planned' && s.match && s.court === 2)!;
    const elsewhere = schedule.slots.find((s) => s.state === 'planned' && s.match && s.court === 1)!;
    const at = (min: number) => new Date(Date.parse(anchor.end) + min * 60_000).toISOString();
    view({ ...schedule, ghosts: [{ id: 'g9', court: 2, minutes: 20, after: anchor.match, start: at(0), end: at(20) }] });
    const f = await render(Schedule);
    const ghost = find(f, '[data-lane="2"] [data-ghost="g9"]');
    expect(ghost.textContent).toContain('Ghost match');
    expect(ghost.textContent).toContain('20 min');
    expect(ghost.getAttribute('title')).toContain(`after ${anchor.match!.id}`);
    expect(find(f, '[data-ghosts]').textContent).toContain('Court 2 · 20 min');

    const [court, after] = [...f.nativeElement.querySelectorAll('[data-ghosts] select')] as HTMLSelectElement[];
    await choose(f, court, '2');
    const options = [...after.options].map((o) => o.value);
    expect(options[0]).toBe('');
    expect(options).toContain(anchor.match!.id);
    expect(options).not.toContain(elsewhere.match!.id);
    await choose(f, after, anchor.match!.id);
    await type(f, find(f, '[data-ghosts] input') as HTMLInputElement, '30');
    await click(f, 'Add a ghost match');
    expect(go.argsOf('TournamentService.AddGhostMatch')).toEqual([[2, 30, anchor.match!.id]]);

    (find(f, '[data-ghosts] .break-remove') as HTMLButtonElement).click();
    (ghost.querySelector('button') as HTMLButtonElement).click();
    expect(go.argsOf('TournamentService.RemoveGhostMatch')).toEqual([['g9'], ['g9']]);
  });

  it('adds a ghost match emptying a court as soon as it is free', async () => {
    const f = await render(Schedule);
    expect(find(f, '[data-ghosts]').textContent).toContain('No ghost match');
    await click(f, 'Add a ghost match');
    expect(go.argsOf('TournamentService.AddGhostMatch')).toEqual([[1, 20, '']]);
  });

  it('folds the order of rounds', async () => {
    const f = await render(Schedule);
    (find(f, '.blocks .fold') as HTMLButtonElement).click();
    f.detectChanges();
    expect(f.nativeElement.querySelector('[data-blocks]')).toBeNull();
    expect(find(f, '.layout').classList).toContain('blocks-folded');
    (find(f, '.strip') as HTMLButtonElement).click();
    f.detectChanges();
    expect(f.nativeElement.querySelector('[data-blocks]')).not.toBeNull();
  });

  it('validates, and asks twice before recomputing or going back', async () => {
    const f = await render(Schedule);
    await click(f, 'Validate the schedule');
    expect(go.called('TournamentService.ValidateSchedule')).toBe(true);
    view(validated);
    go.emit('tournament.updated', 2);
    await new Promise((r) => setTimeout(r, 10));
    f.detectChanges();
    await click(f, 'Recompute');
    expect(go.argsOf('TournamentService.ValidateSchedule')).toHaveLength(1);
    await click(f, 'Recompute');
    expect(go.argsOf('TournamentService.ValidateSchedule')).toHaveLength(2);
    await click(f, 'Back to blocks');
    await click(f, 'Back to blocks');
    expect(go.called('TournamentService.UnvalidateSchedule')).toBe(true);
  });

  it('shares the courts between two categories', async () => {
    const f = await render(Schedule);
    await click(f, 'Reserved courts');
    expect(go.argsOf('TournamentService.SetScheduleMode')).toEqual([['reserved']]);
    await click(f, "Women's doubles");
    expect(go.argsOf('TournamentService.SetScheduleFirst')).toEqual([['WD']]);
  });

  it('lists the rules of each court and flags the courts on the timeline', async () => {
    view(rules);
    const f = await render(Schedule);
    expect(text(f)).toContain('Court rules');
    expect(text(f)).toContain('4 rules');
    const court7 = find(f, '[data-court-rules] [data-court="7"]');
    expect(court7.textContent).toContain('Only');
    expect(court7.textContent).toContain('B · Final');
    expect(find(f, '[data-court="3"]').textContent).toContain('Never');
    expect(find(f, '[data-court="3"]').textContent).toContain('whole category');
    expect(find(f, '[data-court="1"]').textContent).toContain('Priority 1');
    expect(f.nativeElement.querySelector('[data-court="5"]')).toBeNull();
    expect(find(f, '[data-free]').textContent).toContain('Courts 4, 5, 6 have no rule');
    const flags = [...f.nativeElement.querySelectorAll('.court-flag')] as HTMLElement[];
    expect(flags.length).toBe(4);
    expect(flags[0].getAttribute('title')).toContain('Priority 1 WD P · Pools');
  });

  it('greys a rule whose matches are played and warns of matches against the rules', async () => {
    const over = structuredClone(rules);
    over.courtRules[6].rules[0].scope.left = false;
    view({ ...over, offRules: 2, slots: over.slots.map((s, i) => (i === over.slots.length - 1 ? { ...s, offRules: true } : s)) });
    const f = await render(Schedule);
    const chip = find(f, '[data-court="7"] .rule-chip');
    expect(chip.classList).toContain('over');
    expect(chip.textContent).toContain('over');
    expect(find(f, '[data-off-rules]').textContent).toContain('2 matches against the court rules');
    const titles = [...f.nativeElement.querySelectorAll('.slot')].map((e) => (e as HTMLElement).getAttribute('title') ?? '');
    expect(titles.filter((t) => t.includes('against the court rules'))).toHaveLength(1);
  });

  it('previews a rule for several courts, then adds it', async () => {
    const preview = { ...schedule, slots: schedule.slots.slice(0, 3) };
    go.on('TournamentService.PreviewCourtRule', preview);
    const f = await render(Schedule);
    const toggles = [...f.nativeElement.querySelectorAll('.court-toggle')] as HTMLButtonElement[];
    toggles[0].click();
    toggles[1].click();
    f.detectChanges();
    const plus = find(f, '[data-rule-form] .stepper button:last-child') as HTMLButtonElement;
    plus.click();
    f.detectChanges();
    expect(find(f, '[data-level]').textContent).toContain('2');
    const target = find(f, '[data-rule-form] select') as HTMLSelectElement;
    const poolA = schedule.scopes.findIndex((sc) => sc.scope.category === 'WD' && sc.scope.pool === 'A');
    expect([...target.options].map((o) => o.textContent?.trim())).toContain('P A');
    await choose(f, target, String(poolA));
    expect(find(f, '[data-rule-form] .badge').textContent).toContain('WD');
    await new Promise((r) => setTimeout(r, 10));
    f.detectChanges();
    const rule = { kind: 'priority', level: 2, scope: { category: 'WD', stage: 'S1', pool: 'A' } };
    expect(go.argsOf('TournamentService.PreviewCourtRule').at(-1)).toEqual([[1, 2], rule]);
    expect(find(f, '[data-previewing]').textContent).toContain('Preview');
    expect(f.nativeElement.querySelectorAll('.slot').length).toBe(3);

    go.on('TournamentService.AddCourtRule', null);
    await click(f, 'Add the rule');
    expect(go.argsOf('TournamentService.AddCourtRule')).toEqual([[[1, 2], rule]]);
    f.detectChanges();
    expect(f.nativeElement.querySelector('[data-previewing]')).toBeNull();
    expect(f.nativeElement.querySelectorAll('.slot').length).toBe(schedule.slots.length);
  });

  it('adds an only or a never rule, without a level', async () => {
    const f = await render(Schedule);
    expect(text(f)).toContain('Court rules');
    (find(f, '.court-toggle') as HTMLButtonElement).click();
    await click(f, 'Never');
    expect(f.nativeElement.querySelector('[data-level]')).toBeNull();
    expect(text(f)).toContain('the desk can still call one there');
    const target = find(f, '[data-rule-form] select') as HTMLSelectElement;
    await choose(f, target, '0');
    await click(f, 'Cancel');
    expect(f.nativeElement.querySelector('[data-previewing]')).toBeNull();
    expect(f.nativeElement.querySelectorAll('.court-toggle.active')).toHaveLength(0);
    (find(f, '.court-toggle') as HTMLButtonElement).click();
    await click(f, 'Only');
    await choose(f, target, '0');
    await click(f, 'Add the rule');
    expect(go.argsOf('TournamentService.AddCourtRule')).toEqual([[[1], { kind: 'only', level: 0, scope: { category: 'MD' } }]]);
  });

  it('removes a rule, clears a court and folds the panel', async () => {
    view(rules);
    const f = await render(Schedule);
    (find(f, '[data-court="7"] .break-remove') as HTMLButtonElement).click();
    expect(go.argsOf('TournamentService.RemoveCourtRule')).toEqual([[7, { category: 'MD', stage: 'S2', round: 2, from: 1 }]]);
    const withTwo = { ...rules, courtRules: rules.courtRules.map((c) => (c.number === 1 ? { ...c, rules: [...c.rules, ...rules.courtRules[2].rules] } : c)) };
    view(withTwo);
    const g = await render(Schedule);
    await click(g, 'Remove all');
    expect(go.argsOf('TournamentService.ClearCourtRules')).toEqual([[1]]);
    (find(g, '[data-court-rules] [data-fold]') as HTMLButtonElement).click();
    g.detectChanges();
    expect(g.nativeElement.querySelector('[data-rule-form]')).toBeNull();
    expect(text(g)).toContain('5 rules');
  });

  it('names rules in French', async () => {
    view(rules);
    const f = await render(Schedule, { lang: 'fr' });
    expect(find(f, '[data-court="7"]').textContent).toContain('Uniquement');
    expect(find(f, '[data-court="7"]').textContent).toContain('T · Finale');
    expect(find(f, '[data-court="1"]').textContent).toContain('Priorité 1');
    expect(text(f)).toContain('Règles des terrains');
  });

  it('shows an empty schedule before the format', async () => {
    view({ ...before, slots: [], blocks: [] });
    const f = await render(Schedule);
    expect(text(f)).toContain('Nothing to schedule');
    view(before);
    await clickAll(await render(Schedule));
  });

  it('runs every button', async () => {
    await clickAll(await render(Schedule));
  });

  it('previews then moves a round dragged in the list', async () => {
    go.on('TournamentService.PreviewScheduledBlockMove', schedule);
    const f = await render(Schedule);
    const rows = [...f.nativeElement.querySelectorAll('[data-block-index]')] as HTMLElement[];
    await drag(f, rows[3], rows[0]);
    expect(go.called('TournamentService.PreviewScheduledBlockMove')).toBe(true);
    expect(go.argsOf('TournamentService.MoveScheduledBlock')[0]?.[0]).toBe(schedule.blocks[3].key);
  });

  it('previews then moves a match dragged on the board once validated', async () => {
    view(validated);
    go.on('TournamentService.PreviewScheduledMatchMove', validated);
    const f = await render(Schedule);
    const movable = [...f.nativeElement.querySelectorAll('.slot.movable')] as HTMLElement[];
    expect(movable.length).toBeGreaterThan(2);
    await drag(f, movable[movable.length - 1], movable[0]);
    expect(go.called('TournamentService.PreviewScheduledMatchMove')).toBe(true);
    expect(go.called('TournamentService.MoveScheduledMatch')).toBe(true);
    const lane = f.nativeElement.querySelector('[data-lane]') as HTMLElement;
    await drag(f, movable[0], lane, { x: 10, y: 99999 });
    const out = document.createElement('div');
    await drag(f, movable[0], out);
  });
});
