import { Injectable, inject } from '@angular/core';
import { TranslocoService } from '@jsverse/transloco';
import { Events } from '@wailsio/runtime';

export const AVAILABLE_LANGUAGES = ['en', 'fr'] as const;
export type Language = (typeof AVAILABLE_LANGUAGES)[number];
export const DEFAULT_LANGUAGE: Language = 'fr';

const EVENT_LANGUAGE_CHANGED = 'language.changed';

/**
 * Switches the UI language at runtime and keeps both windows in sync.
 *
 * TODO: persist the choice in app settings (app.db) once the store exists.
 */
@Injectable({ providedIn: 'root' })
export class LanguageService {
  private readonly transloco = inject(TranslocoService);

  readonly languages = AVAILABLE_LANGUAGES;

  init(): void {
    Events.On(EVENT_LANGUAGE_CHANGED, (event) => this.apply(event.data));
  }

  get active(): Language {
    return this.transloco.getActiveLang() as Language;
  }

  set(lang: Language): void {
    this.apply(lang);
    void Events.Emit(EVENT_LANGUAGE_CHANGED, lang);
  }

  private apply(lang: string): void {
    if ((AVAILABLE_LANGUAGES as readonly string[]).includes(lang) && lang !== this.active) {
      this.transloco.setActiveLang(lang);
      document.documentElement.lang = lang;
    }
  }
}
