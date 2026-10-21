import { TestBed } from '@angular/core/testing';
import { TournamentView } from '@bindings/apps/desktop';
import { Category, EntrantID, Score } from '@bindings/internal/engine';
import { MatchID, PlayerID, ScheduleMode, StageKind } from '@bindings/internal/tournament';

import { FakeBackend, backendFailure } from '../../testing/backend';
import tournament from '../../testing/fixtures/progress-tournament.json';
import recent from '../../testing/fixtures/recent.json';
import { TournamentStore } from './tournament.service';

const MD = 'MD' as Category;
const e1 = 'e1' as EntrantID;
const m1 = 'MD-A1' as MatchID;

describe('TournamentStore', () => {
  let go: FakeBackend;
  let store: TournamentStore;

  beforeEach(() => {
    go = new FakeBackend()
      .on('TournamentService.GetTournamentView', tournament as unknown as TournamentView)
      .on('TournamentService.ListRecentTournaments', recent);
    TestBed.configureTestingModule({});
    store = TestBed.inject(TournamentStore);
  });

  it('refreshes the tournament and the recent files', async () => {
    await store.refresh();
    expect(store.view().name).toBe(tournament.name);
    expect(store.recent()).toHaveLength(recent.length);
  });

  it('refreshes on tournament.updated', async () => {
    go.emit('tournament.updated', 3);
    await new Promise((r) => setTimeout(r));
    expect(go.called('TournamentService.GetTournamentView')).toBe(true);
  });

  const commands: [string, (s: TournamentStore) => Promise<boolean>, string, unknown[]][] = [
    ['open', (s) => s.open('/t.openshuttle'), 'OpenTournament', ['/t.openshuttle']],
    ['close', (s) => s.close(), 'CloseTournament', []],
    ['rename', (s) => s.rename('Plume'), 'RenameTournament', ['Plume']],
    ['undo', (s) => s.undo(), 'Undo', []],
    ['addCategory', (s) => s.addCategory(MD), 'AddCategory', [MD]],
    ['removeCategory', (s) => s.removeCategory(MD), 'RemoveCategory', [MD]],
    ['registerEntrant', (s) => s.registerEntrant(MD, [{ name: 'A', club: '' }]), 'RegisterEntrant', [MD, [{ name: 'A', club: '' }]]],
    ['updateEntrant', (s) => s.updateEntrant(e1, []), 'UpdateEntrant', [e1, []]],
    ['removeEntrant', (s) => s.removeEntrant(e1), 'RemoveEntrant', [e1]],
    ['checkIn', (s) => s.checkIn('p1' as PlayerID), 'CheckInPlayer', ['p1']],
    ['checkInEntrant', (s) => s.checkInEntrant(e1), 'CheckInEntrant', [e1]],
    ['cancelCheckIn', (s) => s.cancelCheckIn('p1' as PlayerID), 'CancelCheckIn', ['p1']],
    ['setFormat', (s) => s.setFormat(MD, []), 'SetFormat', [MD, []]],
    ['addStage', (s) => s.addStage(MD, StageKind.StageBracket), 'AddStage', [MD, StageKind.StageBracket]],
    ['removeStage', (s) => s.removeStage(MD, 'S2'), 'RemoveStage', [MD, 'S2']],
    ['fillStage', (s) => s.fillStage(MD, 'S2'), 'FillStage', [MD, 'S2']],
    ['swapSlots', (s) => s.swapSlots(MD, 'S2', 0, 3), 'SwapSlots', [MD, 'S2', 0, 3]],
    ['applyFormatPreset', (s) => s.applyFormatPreset(MD, 'b-1'), 'ApplyFormatPreset', [MD, 'b-1']],
    ['saveFormatPreset', (s) => s.saveFormatPreset(MD, 'Mine', 'd'), 'SaveFormatPreset', [MD, 'Mine', 'd']],
    ['setSeeds', (s) => s.setSeeds(MD, [e1]), 'SetSeeds', [MD, [e1]]],
    ['drawEntry', (s) => s.drawEntry(MD), 'DrawEntry', [MD]],
    ['moveEntrant', (s) => s.moveEntrant(e1, 2), 'MoveEntrant', [e1, 2]],
    ['confirmDraw', (s) => s.confirmDraw(MD), 'ConfirmDraw', [MD]],
    ['reopenDraw', (s) => s.reopenDraw(MD), 'ReopenDraw', [MD]],
    ['callMatch', (s) => s.callMatch(m1, 3), 'CallMatch', [m1, 3]],
    ['endMatch', (s) => s.endMatch(m1), 'EndMatch', [m1]],
    ['recordScore', (s) => s.recordScore(m1, { sets: [] } as unknown as Score), 'RecordScore', [m1, { sets: [] }]],
    ['withdrawEntrant', (s) => s.withdrawEntrant(e1), 'WithdrawEntrant', [e1]],
    ['setCourts', (s) => s.setCourts(5), 'SetCourts', [5]],
    ['setMinRest', (s) => s.setMinRest(12), 'SetMinRest', [12]],
    ['setScheduleMode', (s) => s.setScheduleMode(ScheduleMode.ScheduleReserved), 'SetScheduleMode', [ScheduleMode.ScheduleReserved]],
    ['setScheduleFirst', (s) => s.setScheduleFirst(MD), 'SetScheduleFirst', [MD]],
    ['setCourtCategory', (s) => s.setCourtCategory(2, ''), 'SetCourtCategory', [2, '']],
    ['validateSchedule', (s) => s.validateSchedule(), 'ValidateSchedule', []],
    ['moveScheduledMatch', (s) => s.moveScheduledMatch(m1, ''), 'MoveScheduledMatch', [m1, '']],
    ['moveScheduledBlock', (s) => s.moveScheduledBlock('MD/S1/A/1', ''), 'MoveScheduledBlock', ['MD/S1/A/1', '']],
    ['unvalidateSchedule', (s) => s.unvalidateSchedule(), 'UnvalidateSchedule', []],
    ['clearScore', (s) => s.clearScore(m1), 'ClearScore', [m1]],
    ['reinstateEntrant', (s) => s.reinstateEntrant(e1), 'ReinstateEntrant', [e1]],
    ['swapMatches', (s) => s.swapMatches(m1, 'MD-A2' as MatchID), 'SwapMatches', [m1, 'MD-A2']],
    ['drawLots', (s) => s.drawLots(MD, 'S1', 0), 'DrawLots', [MD, 'S1', 0]],
    ['startRound', (s) => s.startRound(), 'StartRound', []],
    ['endRound', (s) => s.endRound(), 'EndRound', []],
  ];

  it.each(commands)('%s calls Go', async (_name, run, binding, args) => {
    expect(await run(store)).toBe(true);
    expect(go.argsOf(`TournamentService.${binding}`)).toEqual([args]);
    expect(store.error()).toBeNull();
  });

  it('keeps the error code of a refused command', async () => {
    go.on('TournamentService.CallMatch', () => {
      throw backendFailure('COURT_BUSY');
    });
    expect(await store.callMatch(m1, 1)).toBe(false);
    expect(store.error()).toEqual({ code: 'COURT_BUSY' });
  });

  it('creates, opens with a dialog and duplicates, false when cancelled', async () => {
    go.on('TournamentService.CreateTournament', '/t.openshuttle').on('TournamentService.OpenTournamentDialog', '');
    expect(await store.create('Plume')).toBe(true);
    expect(await store.openDialog()).toBe(false);
    go.on('TournamentService.DuplicateTournament', '/copy.openshuttle');
    expect(await store.duplicate('/t.openshuttle', 'Copie')).toBe(true);
    expect(go.argsOf('TournamentService.DuplicateTournament')).toEqual([['/t.openshuttle', 'Copie']]);
  });
});
