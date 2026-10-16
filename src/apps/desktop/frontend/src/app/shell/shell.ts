import { Component, HostListener, OnInit, computed, effect, inject, signal } from '@angular/core';
import { Router, RouterOutlet } from '@angular/router';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';

import { DisplayScreenService } from '../core/display-screen.service';
import { Language, LanguageService } from '../core/language.service';
import { TournamentStore } from '../core/tournament.service';
import { ActionView } from '@bindings/apps/desktop';
import { CategoryBadge } from '../shared/category-badge';

/** Main window: top bar, then the home screen or the open tournament. */
@Component({
  selector: 'app-shell',
  imports: [TranslocoDirective, RouterOutlet, CategoryBadge],
  templateUrl: './shell.html',
  styleUrl: './shell.scss',
})
export class Shell implements OnInit {
  protected readonly display = inject(DisplayScreenService);
  protected readonly language = inject(LanguageService);
  protected readonly tournament = inject(TournamentStore);
  private readonly transloco = inject(TranslocoService);
  private readonly router = inject(Router);

  protected readonly monitorId = signal('');
  protected readonly view = this.tournament.view;
  protected readonly lastAction = computed(() => this.view().lastAction);

  constructor() {
    // Opening a tournament enters it; closing it goes back home.
    effect(() => {
      const open = this.view().open;
      const inTournament = this.router.url.startsWith('/tournament');
      if (open && !inTournament) {
        void this.router.navigateByUrl('/tournament');
      } else if (!open && inTournament) {
        void this.router.navigateByUrl('/');
      }
    });
  }

  ngOnInit(): void {
    void this.display.refresh();
    void this.tournament.refresh();
  }

  /**
   * Ctrl Z undoes the latest action, except while typing in a field: there it
   * undoes the typing. An empty field (the check-in search) has nothing to
   * undo, so Ctrl Z goes to the tournament.
   */
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

  /** Translated label of a history entry. */
  protected actionLabel(action: ActionView): string {
    const params = this.translatedParams(action.params ?? {});
    if (action.type !== 'ActionUndone') {
      return this.transloco.translate(`history.${action.type}`, params);
    }
    const undone = this.transloco.translate(`historyShort.${params['action']}`, params);
    return this.transloco.translate('history.ActionUndone', { action: undone });
  }

  private translatedParams(params: Record<string, string | undefined>): Record<string, string> {
    const out: Record<string, string> = {};
    for (const [key, value] of Object.entries(params)) {
      out[key] =
        key === 'category' && value
          ? this.transloco.translate(`category.short.${value}`)
          : (value ?? '');
    }
    return out;
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
  if (target instanceof HTMLInputElement || target instanceof HTMLTextAreaElement) {
    return target.value !== '';
  }
  return target instanceof HTMLElement && target.isContentEditable;
}
