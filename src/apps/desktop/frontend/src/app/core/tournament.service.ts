import { Injectable, signal } from '@angular/core';
import { Events } from '@wailsio/runtime';
import { RecentTournament, TournamentService as Backend, TournamentView } from '@bindings/apps/desktop';
import { Category, EntrantID } from '@bindings/internal/engine';
import { PlayerID, PlayerInput, PoolStage } from '@bindings/internal/tournament';

import { BackendError, backendError } from './backend-error';

const CLOSED: TournamentView = {
  open: false,
  path: '',
  name: '',
  version: 0,
  lastAction: null,
  canUndo: false,
  categories: [],
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

  cancelCheckIn(id: PlayerID): Promise<boolean> {
    return this.command(() => Backend.CancelCheckIn(id));
  }

  configurePoolStage(category: Category, stage: PoolStage): Promise<boolean> {
    return this.command(() => Backend.ConfigurePoolStage(category, stage));
  }

  setSeed(id: EntrantID, seed: number): Promise<boolean> {
    return this.command(() => Backend.SetSeed(id, seed));
  }

  drawPools(category: Category): Promise<boolean> {
    return this.command(() => Backend.DrawPools(category));
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
