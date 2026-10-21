import { TestBed } from '@angular/core/testing';
import { Settings } from '@bindings/apps/desktop';
import { TranslocoService, provideTransloco } from '@jsverse/transloco';
import { of } from 'rxjs';

import { FakeBackend, backendFailure } from '../../testing/backend';
import settings from '../../testing/fixtures/settings.json';
import { LanguageService } from './language.service';

describe('LanguageService', () => {
  let go: FakeBackend;

  function setup(): LanguageService {
    TestBed.configureTestingModule({
      providers: [
        provideTransloco({
          config: { availableLangs: ['en', 'fr'], defaultLang: 'en' },
          loader: class {
            getTranslation() {
              return of({});
            }
          },
        }),
      ],
    });
    return TestBed.inject(LanguageService);
  }

  beforeEach(() => {
    go = new FakeBackend().on('SettingsService.GetSettings', { ...settings, language: 'fr' } as unknown as Settings);
  });

  it('starts in the saved language and follows settings.changed', async () => {
    const lang = setup();
    await lang.init();
    expect(lang.active).toBe('fr');
    expect(document.documentElement.lang).toBe('fr');
    go.emit('settings.changed', { ...settings, language: 'en' });
    expect(lang.active).toBe('en');
    go.emit('settings.changed', { ...settings, language: 'de' });
    expect(lang.active).toBe('en');
  });

  it('follows the system language on first run, or when settings fail', async () => {
    go.on('SettingsService.GetSettings', () => {
      throw backendFailure('INTERNAL');
    });
    const lang = setup();
    await lang.init();
    expect(['en', 'fr']).toContain(lang.active);
  });

  it('saves a new language', async () => {
    const lang = setup();
    await lang.set('fr');
    expect(go.argsOf('SettingsService.SetLanguage')).toEqual([['fr']]);
    expect(TestBed.inject(TranslocoService).getActiveLang()).toBe('fr');
  });

  it('formats times and dates in the UI language', () => {
    const lang = setup();
    expect(lang.time(null)).toBe('');
    expect(lang.time('2026-10-17T14:05:00')).toMatch(/0?2:05|14:05/);
    expect(lang.date('2026-10-17T12:00:00')).toContain('2026');
  });
});
