import { Component, OnInit, computed, effect, inject } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { NavigationEnd, Router, RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { filter, map } from 'rxjs';

import { DisplayScreenService } from '../core/display-screen.service';
import { LanguageService } from '../core/language.service';
import { AppSettings } from '../core/settings.service';
import { SettingsService } from '@bindings/apps/desktop';
import { TournamentStore } from '../core/tournament.service';
import { ActionView } from '@bindings/apps/desktop';
import { ActionLabels } from '../core/action-label';
import { Clock } from '../core/clock';

/**
 * Main window: the top bar, then the home screens (recent tournaments,
 * settings) or the open tournament. Going home keeps the tournament open:
 * the top bar offers to resume it.
 */
@Component({
  selector: 'app-shell',
  imports: [TranslocoDirective, RouterOutlet, RouterLink, RouterLinkActive],
  templateUrl: './shell.html',
  styleUrl: './shell.scss',
})
export class Shell implements OnInit {
  protected readonly display = inject(DisplayScreenService);
  protected readonly language = inject(LanguageService);
  protected readonly tournament = inject(TournamentStore);
  protected readonly clock = inject(Clock);
  private readonly transloco = inject(TranslocoService);
  private readonly router = inject(Router);
  private readonly labels = inject(ActionLabels);

  private readonly settings = inject(AppSettings);
  /** Display monitor, saved in the settings. */
  protected readonly monitorId = computed(() => this.settings.value()?.monitor ?? '');
  protected readonly view = this.tournament.view;
  protected readonly lastAction = computed(() => this.view().lastAction);

  private readonly url = toSignal(
    this.router.events.pipe(
      filter((e) => e instanceof NavigationEnd),
      map(() => this.router.url),
    ),
    { initialValue: this.router.url },
  );
  /** True inside the open tournament, false on the home screens. */
  protected readonly inTournament = computed(() => this.view().open && this.url().startsWith('/tournament'));

  protected readonly homeTabs = [
    { key: 'home', path: '/' },
    { key: 'presets', path: '/presets' },
    { key: 'settings', path: '/settings' },
  ];

  /** Two letters standing for the tournament: "Plume d'automne" → PD. */
  protected readonly mark = computed(() => initials(this.view().name));

  constructor() {
    // Closing the tournament leaves its screens.
    effect(() => {
      if (!this.view().open && this.url().startsWith('/tournament')) {
        void this.router.navigateByUrl('/');
      }
    });
  }

  ngOnInit(): void {
    void this.display.refresh();
    void this.tournament.refresh();
  }

  protected undo(): void {
    void this.tournament.undo();
  }

  protected resume(): void {
    void this.router.navigateByUrl('/tournament');
  }

  /** Translated label of a history entry. */
  protected actionLabel(action: ActionView): string {
    return this.labels.label(action);
  }

  protected time(value: string | Date): string {
    return new Intl.DateTimeFormat(this.language.active, { hour: '2-digit', minute: '2-digit' }).format(
      new Date(value),
    );
  }

  /** Translation key for an error code, INTERNAL for unknown codes. */
  protected errorKey(code: string): string {
    const key = `errors.${code}`;
    return this.transloco.translate(key) === key ? 'errors.INTERNAL' : key;
  }

  protected onMonitor(event: Event): void {
    const id = (event.target as HTMLSelectElement).value;
    void this.settings.run(() => SettingsService.SetMonitor(id));
    if (this.display.isOpen()) {
      void this.display.open(id);
    }
  }

  protected toggleDisplay(): void {
    void (this.display.isOpen() ? this.display.close() : this.display.open(this.monitorId()));
  }
}

/** First letters of the first two words, digits skipped: "Plume d'automne 2026" → PD. */
export function initials(name: string): string {
  const words = name.split(/\s+/).filter((w) => /^\p{L}/u.test(w));
  return words.slice(0, 2).map((w) => w[0].toUpperCase()).join('') || 'OS';
}
