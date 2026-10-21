import { Injectable, inject } from '@angular/core';
import { TranslocoService } from '@jsverse/transloco';
import { Events } from '@wailsio/runtime';
import { SettingsService } from '@bindings/apps/desktop';

export const AVAILABLE_LANGUAGES = ['en', 'fr'] as const;
export type Language = (typeof AVAILABLE_LANGUAGES)[number];
export const DEFAULT_LANGUAGE: Language = 'en';

/**
 * UI language. The choice is stored by Go in app.db; Go broadcasts
 * settings.changed so both windows switch together.
 */
@Injectable({ providedIn: 'root' })
export class LanguageService {
  private readonly transloco = inject(TranslocoService);

  readonly languages = AVAILABLE_LANGUAGES;

  async init(): Promise<void> {
    Events.On('settings.changed', (event) => this.apply(event.data.language));
    try {
      const settings = await SettingsService.GetSettings();
      this.apply(settings.language || systemLanguage());
    } catch {
      this.apply(systemLanguage());
    }
  }

  get active(): Language {
    return this.transloco.getActiveLang() as Language;
  }

  /** "14:05" or "2:05 PM": hours and minutes in the UI language; "" without a time. */
  time(value: string | number | Date | null | undefined): string {
    return value ? new Intl.DateTimeFormat(this.active, { hour: '2-digit', minute: '2-digit' }).format(new Date(value)) : '';
  }

  /** "19 octobre 2026" or "October 19, 2026". */
  date(value: string | number | Date): string {
    return new Intl.DateTimeFormat(this.active, { day: 'numeric', month: 'long', year: 'numeric' }).format(new Date(value));
  }

  async set(lang: Language): Promise<void> {
    await SettingsService.SetLanguage(lang);
    this.apply(lang);
  }

  private apply(lang: string): void {
    if ((AVAILABLE_LANGUAGES as readonly string[]).includes(lang) && lang !== this.active) {
      this.transloco.setActiveLang(lang);
    }
    document.documentElement.lang = this.active;
  }
}

/** First run: follow the system language. */
function systemLanguage(): Language {
  return navigator.language.toLowerCase().startsWith('fr') ? 'fr' : 'en';
}
