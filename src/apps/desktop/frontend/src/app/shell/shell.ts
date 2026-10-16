import { Component, OnInit, inject, signal } from '@angular/core';
import { TranslocoDirective } from '@jsverse/transloco';

import { DisplayScreenService } from '../core/display-screen.service';
import { Language, LanguageService } from '../core/language.service';

/** Main window: top bar and, for now, the home screen. */
@Component({
  selector: 'app-shell',
  imports: [TranslocoDirective],
  templateUrl: './shell.html',
  styleUrl: './shell.scss',
})
export class Shell implements OnInit {
  protected readonly display = inject(DisplayScreenService);
  protected readonly language = inject(LanguageService);
  protected readonly monitorId = signal('');

  ngOnInit(): void {
    void this.display.refresh();
  }

  protected onLanguage(event: Event): void {
    this.language.set((event.target as HTMLSelectElement).value as Language);
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
