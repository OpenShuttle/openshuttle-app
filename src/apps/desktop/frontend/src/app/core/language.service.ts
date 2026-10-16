import { Injectable, inject } from '@angular/core';
import { TranslocoService } from '@jsverse/transloco';
import { Events } from '@wailsio/runtime';
import { SettingsService } from '@bindings/index';

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
