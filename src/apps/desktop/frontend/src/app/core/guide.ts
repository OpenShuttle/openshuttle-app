import { Browser } from '@wailsio/runtime';

/** The guide "a tournament in 10 steps" on the website, in the UI language. */
export function guideUrl(lang: string): string {
  return lang === 'fr' ? 'https://openshuttle.app/guide/' : 'https://openshuttle.app/en/guide/';
}

/** Opens the guide in the system browser: the app itself never needs the network. */
export function openGuide(lang: string): Promise<void> {
  return Browser.OpenURL(guideUrl(lang));
}

/** The website's download page, in the UI language. */
export function downloadUrl(lang: string): string {
  return lang === 'fr' ? 'https://openshuttle.app/telecharger/' : 'https://openshuttle.app/en/download/';
}

/** Opens the download page in the system browser: the app never updates itself. */
export function openDownload(lang: string): Promise<void> {
  return Browser.OpenURL(downloadUrl(lang));
}
