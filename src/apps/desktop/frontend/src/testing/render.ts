import { Component, Provider, Type } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideRouter, Routes } from '@angular/router';
import { provideTransloco, Translation, TranslocoLoader, TranslocoService } from '@jsverse/transloco';
import { provideTranslocoMessageformat } from '@jsverse/transloco-messageformat';
import { Observable, of } from 'rxjs';

import en from '../i18n/en.json';
import fr from '../i18n/fr.json';

/** Where the screens navigate to in tests: anywhere, an empty page. */
@Component({ template: '' })
class Nowhere {}

class InlineLoader implements TranslocoLoader {
  getTranslation(lang: string): Observable<Translation> {
    return of((lang === 'fr' ? fr : en) as Translation);
  }
}

/** Lets pending promises (fake Go answers) resolve, then renders. */
export async function settle<T>(fixture: ComponentFixture<T>, rounds = 3): Promise<void> {
  for (let i = 0; i < rounds; i++) {
    await new Promise((resolve) => setTimeout(resolve));
    fixture.detectChanges();
  }
  await fixture.whenStable();
}

export interface RenderOptions {
  lang?: 'en' | 'fr';
  inputs?: Record<string, unknown>;
  routes?: Routes;
  providers?: Provider[];
}

/**
 * Renders a component with the app's real translations (ICU plurals
 * included) and router; Go is the FakeBackend set up before.
 */
export async function render<T>(component: Type<T>, options: RenderOptions = {}): Promise<ComponentFixture<T>> {
  TestBed.resetTestingModule();
  TestBed.configureTestingModule({
    imports: [component],
    providers: [
      provideRouter([...(options.routes ?? []), { path: '**', component: Nowhere }]),
      provideTransloco({
        config: { availableLangs: ['en', 'fr'], defaultLang: options.lang ?? 'en', reRenderOnLangChange: true, prodMode: true },
        loader: InlineLoader,
      }),
      provideTranslocoMessageformat(),
      ...(options.providers ?? []),
    ],
  });
  const transloco = TestBed.inject(TranslocoService);
  transloco.setActiveLang(options.lang ?? 'en');
  await new Promise<void>((resolve) => transloco.load(options.lang ?? 'en').subscribe(() => resolve()));
  const fixture = TestBed.createComponent(component);
  for (const [name, value] of Object.entries(options.inputs ?? {})) {
    fixture.componentRef.setInput(name, value);
  }
  fixture.detectChanges();
  await settle(fixture);
  return fixture;
}

/** The text of the rendered component, spaces collapsed. */
export function text<T>(fixture: ComponentFixture<T>): string {
  return (fixture.nativeElement as HTMLElement).textContent?.replace(/\s+/g, ' ').trim() ?? '';
}

/** Clicks the first button (or link) whose text contains label. */
export async function click<T>(fixture: ComponentFixture<T>, label: string, selector = 'button, a'): Promise<void> {
  const el = buttons(fixture, selector).find((b) => b.textContent?.includes(label));
  if (!el) {
    throw new Error(`no "${label}" in: ${buttons(fixture, selector).map((b) => b.textContent?.trim()).join(' | ')}`);
  }
  el.click();
  await settle(fixture);
}

/** The buttons (or other elements) of the component. */
export function buttons<T>(fixture: ComponentFixture<T>, selector = 'button'): HTMLElement[] {
  return [...(fixture.nativeElement as HTMLElement).querySelectorAll<HTMLElement>(selector)];
}

/** The first element matching selector, or an error. */
export function find<T>(fixture: ComponentFixture<T>, selector: string): HTMLElement {
  const el = (fixture.nativeElement as HTMLElement).querySelector<HTMLElement>(selector);
  if (!el) {
    throw new Error(`no ${selector}`);
  }
  return el;
}

/**
 * Clicks every enabled button present now, one after the other (re-rendering
 * in between), and fails if a click throws: runs each handler once.
 * Buttons whose text contains one of skip are left alone.
 */
export async function clickAll<T>(fixture: ComponentFixture<T>, skip: string[] = []): Promise<number> {
  const errors: unknown[] = [];
  const onError = (e: ErrorEvent) => errors.push(e.error);
  window.addEventListener('error', onError);
  let clicked = 0;
  try {
    const labels = buttons(fixture).map((b, i) => ({ i, label: b.textContent?.trim() ?? '' }));
    for (const { i, label } of labels) {
      const b = buttons(fixture)[i] as HTMLButtonElement | undefined;
      if (!b || b.disabled || skip.some((s) => label.includes(s))) {
        continue;
      }
      b.click();
      clicked++;
      await settle(fixture, 1);
    }
  } finally {
    window.removeEventListener('error', onError);
  }
  if (errors.length) {
    throw errors[0];
  }
  return clicked;
}

/** Types into an input (or textarea) and fires input and change. */
export async function type<T>(fixture: ComponentFixture<T>, el: HTMLInputElement | HTMLTextAreaElement, value: string): Promise<void> {
  el.value = value;
  el.dispatchEvent(new Event('input'));
  el.dispatchEvent(new Event('change'));
  await settle(fixture);
}

/** Picks an option of a select by value and fires change. */
export async function choose<T>(fixture: ComponentFixture<T>, el: HTMLSelectElement, value: string): Promise<void> {
  el.value = value;
  el.dispatchEvent(new Event('change'));
  await settle(fixture);
}

/**
 * Drags from an element to another with pointer events, as the app's drag
 * and drop expects them: down on from, moves over the document (where
 * elementFromPoint answers to), up. at is where the pointer ends, relative
 * to to's box (jsdom lays nothing out: boxes are all zero).
 */
export async function drag<T>(fixture: ComponentFixture<T>, from: HTMLElement, to: HTMLElement, at = { x: 50, y: 50 }): Promise<void> {
  const original = document.elementFromPoint;
  document.elementFromPoint = () => to;
  try {
    from.dispatchEvent(new MouseEvent('pointerdown', { bubbles: true, button: 0, clientX: 0, clientY: 0 }));
    for (const step of [0.5, 1]) {
      document.dispatchEvent(new MouseEvent('pointermove', { bubbles: true, clientX: at.x * step, clientY: at.y * step }));
      await settle(fixture, 1);
    }
    await new Promise((r) => setTimeout(r, 350)); // the screen settles between two moves
    document.dispatchEvent(new MouseEvent('pointermove', { bubbles: true, clientX: at.x, clientY: at.y + 1 }));
    await settle(fixture);
    document.dispatchEvent(new MouseEvent('pointerup', { bubbles: true, clientX: at.x, clientY: at.y }));
    await settle(fixture);
  } finally {
    document.elementFromPoint = original;
  }
}
