import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { provideHttpClient } from '@angular/common/http';
import { TestBed } from '@angular/core/testing';
import { Route, provideRouter } from '@angular/router';
import { Settings } from '@bindings/apps/desktop';

import { FakeBackend } from '../testing/backend';
import settings from '../testing/fixtures/settings.json';
import { App } from './app';
import { appConfig } from './app.config';
import { routes } from './app.routes';
import { TranslocoHttpLoader } from './core/transloco-loader';

/** Every route of the tree, children included. */
function allRoutes(list: Route[]): Route[] {
  return list.flatMap((r) => [r, ...allRoutes(r.children ?? [])]);
}

describe('App', () => {
  it('creates the app', async () => {
    await TestBed.configureTestingModule({
      imports: [App],
      providers: [provideRouter([])],
    }).compileComponents();

    const fixture = TestBed.createComponent(App);
    expect(fixture.componentInstance).toBeTruthy();
  });

  it('loads a component for every screen', async () => {
    const lazy = allRoutes(routes).filter((r) => r.loadComponent);
    expect(lazy.length).toBeGreaterThan(10);
    for (const r of lazy) {
      expect(await r.loadComponent!()).toBeTruthy();
    }
  });

  it('configures routing, translations and the language at start', () => {
    expect(appConfig.providers.length).toBeGreaterThan(4);
  });

  it('loads the translations of a language over HTTP', () => {
    new FakeBackend().on('SettingsService.GetSettings', settings as unknown as Settings);
    TestBed.configureTestingModule({ providers: [provideHttpClient(), provideHttpClientTesting()] });
    const loader = TestBed.inject(TranslocoHttpLoader);
    let got: unknown;
    loader.getTranslation('fr').subscribe((t) => (got = t));
    TestBed.inject(HttpTestingController).expectOne('i18n/fr.json').flush({ app: { name: 'OpenShuttle' } });
    expect(got).toEqual({ app: { name: 'OpenShuttle' } });
  });
});
