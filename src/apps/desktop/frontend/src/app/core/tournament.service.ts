import { Injectable, signal } from '@angular/core';
import { Events } from '@wailsio/runtime';
import { RecentTournament, TournamentService as Backend, TournamentView } from '@bindings/apps/desktop';
import { Category, EntrantID, Score } from '@bindings/internal/engine';
import { MatchID, PlayerID, PlayerInput, ScheduleMode, StageConfig, StageKind } from '@bindings/internal/tournament';

import { BackendError, backendError } from './backend-error';

const CLOSED: TournamentView = {
  open: false,
  path: '',
  name: '',
  version: 0,
  lastAction: null,
  canUndo: false,
  archived: false,
  categories: [],
  availableCategories: [],
  courts: 4,
  minRest: 10,
  progress: {
    entrants: 0,
    doubles: false,
    players: 0,
    playersPresent: 0,
    drawsConfirmed: 0,
    drawConfirmedAt: null,
    started: false,
    matches: 0,
    played: 0,
    final: false,
    stage: 'preparation',
  },
};

/**
 * The open tournament, as Go sees it. Commands wait for Go, then the view is
 * refetched on tournament.updated: no optimistic updates.
 */
@Injectable({ providedIn: 'root' })
export class TournamentStore {
  readonly view = signal<TournamentView>(CLOSED);
  readonly recent = signal<RecentTournament[]>([]);
  /** Latest failed command, shown until dismissed. */
  readonly error = signal<BackendError | null>(null);

  constructor() {
    Events.On('tournament.updated', () => void this.refresh());
  }

  async refresh(): Promise<void> {
    const [view, recent] = await Promise.all([
      Backend.GetTournamentView(),
      Backend.ListRecentTournaments(),
    ]);
    this.view.set(view);
    this.recent.set(recent ?? []);
  }

  /** Returns false when the user cancelled the save dialog or it failed. */
  async create(name: string): Promise<boolean> {
    return this.run(async () => (await Backend.CreateTournament(name)) !== '');
  }

  openDialog(): Promise<boolean> {
    return this.run(async () => (await Backend.OpenTournamentDialog()) !== '');
  }

  open(path: string): Promise<boolean> {
    return this.run(async () => {
      await Backend.OpenTournament(path);
      return true;
    });
  }

  /** Copies a tournament's setup, without its entrants. False when cancelled. */
  async duplicate(path: string, name: string): Promise<boolean> {
    const done = await this.run(async () => (await Backend.DuplicateTournament(path, name)) !== '');
    await this.refresh();
    return done;
  }

  close(): Promise<boolean> {
    return this.run(async () => {
      await Backend.CloseTournament();
      return true;
    });
  }

  rename(name: string): Promise<boolean> {
    return this.run(async () => {
      await Backend.RenameTournament(name);
      return true;
    });
  }

  undo(): Promise<boolean> {
    return this.run(async () => {
      await Backend.Undo();
      return true;
    });
  }

  addCategory(category: Category): Promise<boolean> {
    return this.command(() => Backend.AddCategory(category));
  }

  removeCategory(category: Category): Promise<boolean> {
    return this.command(() => Backend.RemoveCategory(category));
  }

  registerEntrant(category: Category, players: PlayerInput[]): Promise<boolean> {
    return this.command(() => Backend.RegisterEntrant(category, players));
  }

  updateEntrant(id: EntrantID, players: PlayerInput[]): Promise<boolean> {
    return this.command(() => Backend.UpdateEntrant(id, players));
  }

  removeEntrant(id: EntrantID): Promise<boolean> {
    return this.command(() => Backend.RemoveEntrant(id));
  }

  checkIn(id: PlayerID): Promise<boolean> {
    return this.command(() => Backend.CheckInPlayer(id));
  }

  checkInEntrant(id: EntrantID): Promise<boolean> {
    return this.command(() => Backend.CheckInEntrant(id));
  }

  cancelCheckIn(id: PlayerID): Promise<boolean> {
    return this.command(() => Backend.CancelCheckIn(id));
  }

  setFormat(category: Category, stages: StageConfig[]): Promise<boolean> {
    return this.command(() => Backend.SetFormat(category, stages));
  }

  addStage(category: Category, kind: StageKind): Promise<boolean> {
    return this.command(() => Backend.AddStage(category, kind));
  }

  removeStage(category: Category, id: string): Promise<boolean> {
    return this.command(() => Backend.RemoveStage(category, id));
  }

  fillStage(category: Category, id: string): Promise<boolean> {
    return this.command(() => Backend.FillStage(category, id));
  }

  swapSlots(category: Category, id: string, a: number, b: number): Promise<boolean> {
    return this.command(() => Backend.SwapSlots(category, id, a, b));
  }

  applyFormatPreset(category: Category, id: string): Promise<boolean> {
    return this.command(() => Backend.ApplyFormatPreset(category, id));
  }

  saveFormatPreset(category: Category, name: string, description: string): Promise<boolean> {
    return this.command(() => Backend.SaveFormatPreset(category, name, description));
  }

  setSeeds(category: Category, ranking: EntrantID[]): Promise<boolean> {
    return this.command(() => Backend.SetSeeds(category, ranking));
  }

  drawEntry(category: Category): Promise<boolean> {
    return this.command(() => Backend.DrawEntry(category));
  }

  moveEntrant(id: EntrantID, pool: number): Promise<boolean> {
    return this.command(() => Backend.MoveEntrant(id, pool));
  }

  confirmDraw(category: Category): Promise<boolean> {
    return this.command(() => Backend.ConfirmDraw(category));
  }

  reopenDraw(category: Category): Promise<boolean> {
    return this.command(() => Backend.ReopenDraw(category));
  }

  callMatch(id: MatchID, court: number): Promise<boolean> {
    return this.command(() => Backend.CallMatch(id, court));
  }

  endMatch(id: MatchID): Promise<boolean> {
    return this.command(() => Backend.EndMatch(id));
  }

  recordScore(id: MatchID, score: Score): Promise<boolean> {
    return this.command(() => Backend.RecordScore(id, score));
  }

  withdrawEntrant(id: EntrantID): Promise<boolean> {
    return this.command(() => Backend.WithdrawEntrant(id));
  }

  setCourts(courts: number): Promise<boolean> {
    return this.command(() => Backend.SetCourts(courts));
  }

  setMinRest(minutes: number): Promise<boolean> {
    return this.command(() => Backend.SetMinRest(minutes));
  }

  setScheduleMode(mode: ScheduleMode): Promise<boolean> {
    return this.command(() => Backend.SetScheduleMode(mode));
  }

  setScheduleFirst(category: Category): Promise<boolean> {
    return this.command(() => Backend.SetScheduleFirst(category));
  }

  setCourtCategory(court: number, category: Category | ''): Promise<boolean> {
    return this.command(() => Backend.SetCourtCategory(court, category as Category));
  }

  validateSchedule(): Promise<boolean> {
    return this.command(() => Backend.ValidateSchedule());
  }

  moveScheduledMatch(id: MatchID, before: MatchID | ''): Promise<boolean> {
    return this.command(() => Backend.MoveScheduledMatch(id, before as MatchID));
  }

  moveScheduledBlock(key: string, before: string): Promise<boolean> {
    return this.command(() => Backend.MoveScheduledBlock(key, before));
  }

  unvalidateSchedule(): Promise<boolean> {
    return this.command(() => Backend.UnvalidateSchedule());
  }

  clearScore(id: MatchID): Promise<boolean> {
    return this.command(() => Backend.ClearScore(id));
  }

  reinstateEntrant(id: EntrantID): Promise<boolean> {
    return this.command(() => Backend.ReinstateEntrant(id));
  }

  swapMatches(a: MatchID, b: MatchID): Promise<boolean> {
    return this.command(() => Backend.SwapMatches(a, b));
  }

  drawLots(category: Category, stage: string, pool: number): Promise<boolean> {
    return this.command(() => Backend.DrawLots(category, stage, pool));
  }

  startRound(): Promise<boolean> {
    return this.command(() => Backend.StartRound());
  }

  endRound(): Promise<boolean> {
    return this.command(() => Backend.EndRound());
  }

  /** Runs a command that returns nothing; true when it succeeded. */
  private command(call: () => Promise<void>): Promise<boolean> {
    return this.run(async () => {
      await call();
      return true;
    });
  }

  private async run(command: () => Promise<boolean>): Promise<boolean> {
    try {
      const done = await command();
      this.error.set(null);
      return done;
    } catch (err) {
      this.error.set(backendError(err));
      return false;
    }
  }
}
