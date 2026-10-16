import { Component, HostListener, OnInit, computed, inject, signal } from '@angular/core';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';

import { DisplayScreenService } from '../core/display-screen.service';
import { Language, LanguageService } from '../core/language.service';
import { TournamentStore } from '../core/tournament.service';
import { Home } from '../home/home';
import { TournamentInfo } from '../tournament-info/tournament-info';

/** Main window: top bar, then the home screen or the open tournament. */
@Component({
  selector: 'app-shell',
  imports: [TranslocoDirective, Home, TournamentInfo],
  templateUrl: './shell.html',
  styleUrl: './shell.scss',
})
export class Shell implements OnInit {
  protected readonly display = inject(DisplayScreenService);
  protected readonly language = inject(LanguageService);
  protected readonly tournament = inject(TournamentStore);
  private readonly transloco = inject(TranslocoService);

  protected readonly monitorId = signal('');
  protected readonly view = this.tournament.view;
  protected readonly lastAction = computed(() => this.view().lastAction);

  ngOnInit(): void {
    void this.display.refresh();
    void this.tournament.refresh();
  }

  /** Ctrl Z undoes the latest action, except while typing in a field. */
  @HostListener('document:keydown', ['$event'])
  protected onKeydown(event: KeyboardEvent): void {
    const isUndo =
      (event.ctrlKey || event.metaKey) && !event.shiftKey && event.key.toLowerCase() === 'z';
    if (!isUndo || isEditable(event.target) || !this.view().canUndo) {
      return;
    }
    event.preventDefault();
    void this.tournament.undo();
  }

  protected undo(): void {
    void this.tournament.undo();
  }

  protected time(iso: string): string {
    return new Intl.DateTimeFormat(this.language.active, { hour: '2-digit', minute: '2-digit' }).format(
      new Date(iso),
    );
  }

  /** Translation key for an error code, INTERNAL for unknown codes. */
  protected errorKey(code: string): string {
    const key = `errors.${code}`;
    return this.transloco.translate(key) === key ? 'errors.INTERNAL' : key;
  }

  protected onLanguage(event: Event): void {
    void this.language.set((event.target as HTMLSelectElement).value as Language);
  }

  protected onMonitor(event: Event): void {
    this.monitorId.set((event.target as HTMLSelectElement).value);
    if (this.display.isOpen()) {
      void this.display.open(this.monitorId());
    }
  }

  protected toggleDisplay(): void {
    void (this.display.isOpen() ? this.display.close() : this.display.open(this.monitorId()));
  }
}

function isEditable(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) {
    return false;
  }
  return target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName);
}
