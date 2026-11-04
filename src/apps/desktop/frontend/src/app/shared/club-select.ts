import { Component, ElementRef, computed, inject, input, output, signal, viewChild } from '@angular/core';
import { TranslocoDirective } from '@jsverse/transloco';
import { PresetService } from '@bindings/apps/desktop';
import { Club } from '@bindings/internal/tournament';

import { AppPresets } from '../core/presets.service';

const NEW = '__new';

const collapse = (s: string): string => s.trim().replace(/\s+/g, ' ');

/**
 * What club names and acronyms are compared on, as Go does
 * (tournament.ClubKey): letters and digits, lowercased, accents dropped.
 */
export function clubKey(s: string): string {
  return s
    .normalize('NFD')
    .replace(/\p{Mn}/gu, '')
    .toLowerCase()
    .replace(/[^\p{L}\p{N}]/gu, '');
}

/** The listed club a value names, by name or acronym. */
export function findClub(list: readonly Club[], value: string): Club | undefined {
  const k = clubKey(value);
  return k ? (list.find((c) => clubKey(c.name) === k) ?? list.find((c) => c.acronym && clubKey(c.acronym) === k)) : undefined;
}

/**
 * Picks a player's club from the club list of this PC, or adds a new club
 * (name and optional acronym) to the list on the spot.
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
            [attr.aria-label]="t('clubs.newPlaceholder')"
            [placeholder]="t('clubs.newPlaceholder')"
            [value]="newName()"
            (input)="newName.set(text($event))"
            (keydown.enter)="$event.preventDefault(); create()"
            (keydown.escape)="$event.stopPropagation(); adding.set(false)"
          />
          <input
            class="input acronym"
            maxlength="20"
            [attr.aria-label]="t('clubs.acronym')"
            [placeholder]="t('clubs.acronymPlaceholder')"
            [value]="newAcronym()"
            (input)="newAcronym.set(text($event))"
            (keydown.enter)="$event.preventDefault(); create()"
            (keydown.escape)="$event.stopPropagation(); adding.set(false)"
          />
          <button type="button" class="btn btn-sm btn-primary" [disabled]="!newName().trim() && !newAcronym().trim()" (click)="create()">
            {{ t('clubs.add') }}
          </button>
          <button type="button" class="btn btn-sm" [attr.aria-label]="t('common.cancel')" (click)="adding.set(false)">×</button>
        </span>
        @if (error(); as code) {
          <span class="error" role="alert">{{ t('errors.' + code) }}</span>
        }
      } @else {
        <select [attr.aria-label]="t('entrants.club')" (change)="pick($event)">
          <option value="" [selected]="!value()">{{ t('clubs.none') }}</option>
          @for (c of clubs(); track c.name) {
            <option [value]="c.name" [selected]="c.name === value()">{{ c.acronym ? c.name + ' (' + c.acronym + ')' : c.name }}</option>
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
      .input.acronym {
        flex: 0 1 6rem;
      }
    }
    .error {
      display: block;
      margin-top: 4px;
      color: var(--color-alert);
      font-size: 12px;
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
  protected readonly newAcronym = signal('');
  protected readonly error = signal<string | null>(null);
  private readonly newField = viewChild<ElementRef<HTMLInputElement>>('newField');

  /** The club list, with the current club even when it is not listed. */
  protected readonly clubs = computed((): Club[] => {
    const list = this.presets.value().clubs ?? [];
    const v = this.value();
    return v && !list.some((c) => c.name === v) ? [{ name: v, acronym: '' }, ...list] : list;
  });

  protected pick(event: Event): void {
    const select = event.target as HTMLSelectElement;
    if (select.value !== NEW) {
      this.changed.emit(select.value);
      return;
    }
    select.value = this.value();
    this.newName.set('');
    this.newAcronym.set('');
    this.error.set(null);
    this.adding.set(true);
    setTimeout(() => this.newField()?.nativeElement.focus());
  }

  protected async create(): Promise<void> {
    const club = { name: collapse(this.newName()), acronym: collapse(this.newAcronym()) };
    const typed = club.name || club.acronym;
    if (!typed) {
      return;
    }
    // A club already listed under that name or acronym is picked as is.
    const listed = club.name && club.acronym ? undefined : findClub(this.presets.value().clubs ?? [], typed);
    if (listed || (await this.presets.run(() => PresetService.SaveClub('', club)))) {
      this.changed.emit(listed?.name ?? typed);
      this.adding.set(false);
      return;
    }
    this.error.set(this.presets.error()?.code ?? null);
  }

  protected text(event: Event): string {
    return (event.target as HTMLInputElement).value;
  }
}
