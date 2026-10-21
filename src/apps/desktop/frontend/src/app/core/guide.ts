import { Browser } from '@wailsio/runtime';

/** The guide "a tournament in 10 steps" on the website, in the UI language. */
export function guideUrl(lang: string): string {
  return lang === 'fr' ? 'https://openshuttle.app/guide/' : 'https://openshuttle.app/en/guide/';
}

/** Opens the guide in the system browser: the app itself never needs the network. */
export function openGuide(lang: string): Promise<void> {
  return Browser.OpenURL(guideUrl(lang));
}
