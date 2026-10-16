import { Location } from '@angular/common';
import { Component, inject, signal } from '@angular/core';
import { TranslocoDirective } from '@jsverse/transloco';
import { AppInfo, AppService, SettingsService } from '@bindings/apps/desktop';

import { DisplayScreenService } from '../core/display-screen.service';
import { Language, LanguageService } from '../core/language.service';
import { AppSettings } from '../core/settings.service';

/** Settings: stored on this PC, for every tournament. */
@Component({
  selector: 'app-settings',
  imports: [TranslocoDirective],
  templateUrl: './settings.html',
  styleUrl: './settings.scss',
})
export class SettingsPage {
  protected readonly settings = inject(AppSettings);
  protected readonly language = inject(LanguageService);
  protected readonly display = inject(DisplayScreenService);
  private readonly location = inject(Location);
  protected readonly app = signal<AppInfo | null>(null);

  constructor() {
    void this.display.refresh();
    void AppService.GetAppInfo().then((info) => this.app.set(info));
  }

  protected back(): void {
    this.location.back();
  }

  protected setLanguage(lang: Language): void {
    void this.settings.run(() => this.language.set(lang));
  }

  protected setMonitor(event: Event): void {
    const id = (event.target as HTMLSelectElement).value;
    void this.settings.run(() => SettingsService.SetMonitor(id));
    if (this.display.isOpen()) {
      void this.display.open(id);
    }
  }

  protected setOperator(event: Event): void {
    const name = (event.target as HTMLInputElement).value;
    if (name !== this.settings.value()?.operator) {
      void this.settings.run(() => SettingsService.SetOperator(name));
    }
  }

  protected step(key: 'rotation' | 'courts' | 'rest', delta: number): void {
    const s = this.settings.value();
    if (!s) {
      return;
    }
    switch (key) {
      case 'rotation':
        void this.settings.run(() => SettingsService.SetRotationSeconds(s.rotationSeconds + delta));
        break;
      case 'courts':
        void this.settings.run(() => SettingsService.SetDefaultCourts(s.defaultCourts + delta));
        break;
      case 'rest':
        void this.settings.run(() => SettingsService.SetDefaultMinRest(s.defaultMinRest + delta));
        break;
    }
  }

  protected setPoolWin(kind: 'sets' | 'timed'): void {
    void this.settings.run(() => SettingsService.SetDefaultPoolWin(kind));
  }

  protected chooseFolder(): void {
    void this.settings.run(() => SettingsService.ChooseTournamentsDir());
  }
}
