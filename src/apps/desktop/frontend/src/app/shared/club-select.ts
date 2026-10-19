import { Component, ElementRef, computed, inject, input, output, signal, viewChild } from '@angular/core';
import { TranslocoDirective } from '@jsverse/transloco';
import { PresetService } from '@bindings/apps/desktop';

import { AppPresets } from '../core/presets.service';

const NEW = '__new';

/**
 * Picks a player's club from the club list of this PC, or adds a new club
 * to the list on the spot.
 */
@Component({
  selector: 'app-club-select',
  imports: [TranslocoDirective],
  template: `
    <ng-container *transloco="let t">
      @if (adding()) {
        <span class="new">
          <input
            #newField
            class="input"
            maxlength="80"
            [placeholder]="t('clubs.newPlaceholder')"
            [value]="newName()"
            (input)="newName.set(text($event))"
            (keydown.enter)="$event.preventDefault(); create()"
            (keydown.escape)="$event.stopPropagation(); adding.set(false)"
          />
          <button type="button" class="btn btn-sm btn-primary" [disabled]="!newName().trim()" (click)="create()">
            {{ t('clubs.add') }}
          </button>
          <button type="button" class="btn btn-sm" [attr.aria-label]="t('common.cancel')" (click)="adding.set(false)">×</button>
        </span>
      } @else {
        <select [attr.aria-label]="t('entrants.club')" (change)="pick($event)">
          <option value="" [selected]="!value()">{{ t('clubs.none') }}</option>
          @for (c of clubs(); track c) {
            <option [value]="c" [selected]="c === value()">{{ c }}</option>
          }
          <option [value]="newOption">+ {{ t('clubs.new') }}</option>
        </select>
      }
    </ng-container>
  `,
  styles: `
    :host {
      display: block;
      min-width: 0;
    }
    select {
      width: 100%;
    }
    .new {
      display: flex;
      gap: 6px;
      .input {
        flex: 1;
        min-width: 0;
      }
    }
  `,
})
export class ClubSelect {
  readonly value = input('');
  readonly changed = output<string>();

  private readonly presets = inject(AppPresets);
  protected readonly newOption = NEW;
  protected readonly adding = signal(false);
  protected readonly newName = signal('');
  private readonly newField = viewChild<ElementRef<HTMLInputElement>>('newField');

  /** The club list, with the current club even when it is not listed. */
  protected readonly clubs = computed(() => {
    const list = this.presets.value().clubs ?? [];
    const v = this.value();
    return v && !list.includes(v) ? [v, ...list] : list;
  });

  protected pick(event: Event): void {
    const select = event.target as HTMLSelectElement;
    if (select.value !== NEW) {
      this.changed.emit(select.value);
      return;
    }
    select.value = this.value();
    this.newName.set('');
    this.adding.set(true);
    setTimeout(() => this.newField()?.nativeElement.focus());
  }

  protected async create(): Promise<void> {
    const name = this.newName().trim().replace(/\s+/g, ' ');
    if (name && (await this.presets.run(() => PresetService.SaveClub('', name)))) {
      // The list matches clubs case-insensitively: pick the listed spelling.
      const listed = (this.presets.value().clubs ?? []).find((c) => c.localeCompare(name, undefined, { sensitivity: 'base' }) === 0);
      this.changed.emit(listed ?? name);
      this.adding.set(false);
    }
  }

  protected text(event: Event): string {
    return (event.target as HTMLInputElement).value;
  }
}
