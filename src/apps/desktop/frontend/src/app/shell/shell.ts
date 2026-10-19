import { Component, DestroyRef, OnInit, computed, effect, inject, signal } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { NavigationEnd, Router, RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { filter, map } from 'rxjs';
import { System, Window } from '@wailsio/runtime';

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

  // The top bar is the window's title bar (see main.go): it drags the
  // window; on Windows and Linux it also holds the window buttons, on macOS
  // the native traffic lights sit at its left.
  protected readonly isMac = System.IsMac();
  protected readonly maximised = signal(false);

  constructor() {
    const refreshMaximised = () => void Window.IsMaximised().then((m) => this.maximised.set(m), () => undefined);
    refreshMaximised();
    window.addEventListener('resize', refreshMaximised);
    inject(DestroyRef).onDestroy(() => window.removeEventListener('resize', refreshMaximised));

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

  protected minimise(): void {
    void Window.Minimise();
  }

  protected toggleMaximise(): void {
    void Window.ToggleMaximise();
  }

  protected close(): void {
    void Window.Close();
  }

  /** Double-click on the bar, outside its controls, maximises or restores the window. */
  protected onTitleDoubleClick(event: MouseEvent): void {
    if (!(event.target as HTMLElement).closest('button, a, select, input')) {
      this.toggleMaximise();
    }
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
