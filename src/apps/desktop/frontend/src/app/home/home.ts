import { Component, ElementRef, computed, effect, inject, signal, viewChild } from '@angular/core';
import { Router } from '@angular/router';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';

import { openGuide } from '../core/guide';
import { LanguageService } from '../core/language.service';
import { AppSettings } from '../core/settings.service';
import { TournamentStore } from '../core/tournament.service';
import { AppInfo, AppService, RecentTournament } from '@bindings/apps/desktop';

/** Home screen: recent tournament files, create, open, duplicate, and the trash. */
@Component({
  selector: 'app-home',
  imports: [TranslocoDirective],
  templateUrl: './home.html',
  styleUrl: './home.scss',
})
export class Home {
  protected readonly tournament = inject(TournamentStore);
  private readonly language = inject(LanguageService);
  private readonly settings = inject(AppSettings);
  private readonly transloco = inject(TranslocoService);
  private readonly router = inject(Router);

  protected readonly app = signal<AppInfo | null>(null);
  protected readonly creating = signal(false);
  protected readonly name = signal('');
  protected readonly folder = computed(() => this.settings.value()?.tournamentsDir ?? '');
  private readonly nameField = viewChild<ElementRef<HTMLInputElement>>('nameField');

  constructor() {
    // The form opens on the desk's click: its field takes the focus.
    effect(() => this.nameField()?.nativeElement.focus());
    void AppService.GetAppInfo().then((info) => this.app.set(info));
    void this.tournament.refresh();
  }

  protected startCreate(): void {
    this.name.set('');
    this.creating.set(true);
  }

  protected async create(): Promise<void> {
    if (await this.tournament.create(this.name())) {
      this.creating.set(false);
      void this.router.navigateByUrl('/tournament');
    }
  }

  /** Building the sample tournament takes a moment: the button waits. */
  protected readonly preparingSample = signal(false);

  protected async trySample(): Promise<void> {
    this.preparingSample.set(true);
    const done = await this.tournament.createSample(this.transloco.translate('home.sampleName'));
    this.preparingSample.set(false);
    if (done) {
      void this.router.navigateByUrl('/tournament/control-desk');
    }
  }

  protected guide(): void {
    void openGuide(this.language.active);
  }

  protected async openDialog(): Promise<void> {
    if (await this.tournament.openDialog()) {
      void this.router.navigateByUrl('/tournament');
    }
  }

  /** Opens a recent tournament; the open one is resumed as it is. */
  protected async open(r: RecentTournament): Promise<void> {
    const v = this.tournament.view();
    if ((v.open && v.path === r.path) || (await this.tournament.open(r.path))) {
      void this.router.navigateByUrl('/tournament');
    }
  }

  protected duplicate(r: RecentTournament): void {
    void this.tournament.duplicate(r.path, this.transloco.translate('home.copyName', { name: r.name }));
  }

  /** The trash view replaces the recent list. */
  protected readonly showTrash = signal(false);
  /** The trashed tournament whose permanent deletion is being confirmed. */
  protected readonly deleting = signal<string | null>(null);
  /** Off by default: deleting from the trash keeps the file unless asked. */
  protected readonly deleteFile = signal(false);

  protected moveToTrash(r: RecentTournament): void {
    void this.tournament.moveToTrash(r.path);
  }

  protected restore(r: RecentTournament): void {
    void this.tournament.restore(r.path);
  }

  protected askDelete(r: RecentTournament): void {
    this.deleteFile.set(false);
    this.deleting.set(r.path);
  }

  protected async confirmDelete(r: RecentTournament): Promise<void> {
    if (await this.tournament.deleteForGood(r.path, r.exists && this.deleteFile())) {
      this.deleting.set(null);
    }
  }

  protected fileName(path: string): string {
    return path.split(/[\\/]/).pop() ?? path;
  }

  protected isOpen(r: RecentTournament): boolean {
    const v = this.tournament.view();
    return v.open && v.path === r.path;
  }

  protected date(iso: string): string {
    return this.language.date(iso);
  }
}
