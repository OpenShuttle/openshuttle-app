import { RecentTournament, TournamentView } from '@bindings/apps/desktop';

import { FakeBackend } from '../../testing/backend';
import { buttons, click, clickAll, find, render, text, type } from '../../testing/render';
import recent from '../../testing/fixtures/recent.json';
import tournament from '../../testing/fixtures/finished-tournament.json';
import { Home } from './home';

describe('Home', () => {
  let go: FakeBackend;

  beforeEach(() => {
    go = new FakeBackend()
      .on('TournamentService.GetTournamentView', { ...tournament, open: false } as unknown as TournamentView)
      .on('TournamentService.ListRecentTournaments', recent as unknown as RecentTournament[]);
  });

  it('lists the recent tournaments and opens one', async () => {
    const f = await render(Home);
    expect(text(f)).toContain(recent[0].name);
    await click(f, 'Open');
  });

  it('creates a tournament', async () => {
    go.on('TournamentService.CreateTournament', '/t.openshuttle');
    const f = await render(Home);
    await click(f, 'New tournament');
    await type(f, find(f, 'form input') as HTMLInputElement, 'Plume');
    (find(f, 'form') as HTMLFormElement).dispatchEvent(new Event('submit'));
    await new Promise((r) => setTimeout(r));
    expect(go.argsOf('TournamentService.CreateTournament')).toEqual([['Plume']]);
  });

  it('says when a newer version is published, with the way to download it', async () => {
    go.on('AppService.GetAppInfo', { version: '1.2.0', compat: 1 });
    go.on('AppService.GetUpdate', { checked: true, latest: '1.3.0', available: true });
    const f = await render(Home);
    expect(text(f)).toContain('OpenShuttle 1.3.0 is available: you are running 1.2.0.');
    await click(f, 'Download');
  });

  it('says nothing when the version is the latest', async () => {
    go.on('AppService.GetAppInfo', { version: '1.3.0', compat: 1 });
    go.on('AppService.GetUpdate', { checked: true, latest: '1.3.0', available: false });
    const f = await render(Home);
    expect(text(f)).not.toContain('is available');
  });

  it('runs every button', async () => {
    go.on('TournamentService.DuplicateTournament', '');
    await clickAll(await render(Home));
  });

  it('builds the sample tournament and opens its control desk', async () => {
    go.on('TournamentService.CreateSampleTournament', '/home/julie/Tournoi d\'exemple.openshuttle');
    const f = await render(Home);
    await click(f, 'Try with a sample tournament');
    expect(go.argsOf('TournamentService.CreateSampleTournament')).toEqual([['Sample tournament']]);
  });

  it('offers the sample tournament and the guide on first launch', async () => {
    go.on('TournamentService.ListRecentTournaments', []);
    const f = await render(Home, { lang: 'fr' });
    expect(text(f)).toContain("Essayer avec un tournoi d'exemple");
    await click(f, 'Lire le guide');
    await click(f, "Essayer avec un tournoi d'exemple");
    expect(go.argsOf('TournamentService.CreateSampleTournament')).toEqual([["Tournoi d'exemple"]]);
  });

  describe('trash', () => {
    const path = recent[0].path;
    const trashed = [{ ...recent[0], trashedAt: '2026-10-20T08:00:00Z' }];

    it('moves a recent tournament to the trash', async () => {
      const f = await render(Home);
      expect(text(f)).not.toContain('Trash ·');
      find(f, 'button[aria-label="Move to the trash"]').click();
      await new Promise((r) => setTimeout(r));
      expect(go.argsOf('TournamentService.TrashTournament')).toEqual([[path]]);
    });

    it('restores a tournament from the trash', async () => {
      go.on('TournamentService.ListTrashedTournaments', trashed);
      const f = await render(Home);
      await click(f, 'Trash · 1');
      expect(text(f)).toContain('Their files stay on this PC');
      expect(text(f)).toContain('Trashed on');
      await click(f, 'Restore');
      expect(go.argsOf('TournamentService.RestoreTournament')).toEqual([[path]]);
      await click(f, 'Back to recent tournaments');
      expect(text(f)).toContain('Recent tournaments');
    });

    it('deletes for good, keeping the file unless asked', async () => {
      go.on('TournamentService.ListTrashedTournaments', trashed);
      const f = await render(Home, { lang: 'fr' });
      await click(f, 'Corbeille · 1');
      await click(f, 'Supprimer…');
      expect(text(f)).toContain(`Supprimer définitivement « ${recent[0].name} » ?`);
      const box = find(f, '.confirm-delete input[type=checkbox]') as HTMLInputElement;
      expect(box.checked).toBe(false);
      await click(f, 'Supprimer de la corbeille');
      expect(go.argsOf('TournamentService.DeleteTournament')).toEqual([[path, false]]);
    });

    it('deletes the file too when asked', async () => {
      go.on('TournamentService.ListTrashedTournaments', trashed);
      const f = await render(Home);
      await click(f, 'Trash · 1');
      await click(f, 'Delete…');
      const box = find(f, '.confirm-delete input[type=checkbox]') as HTMLInputElement;
      box.checked = true;
      box.dispatchEvent(new Event('change'));
      f.detectChanges();
      expect(text(f)).toContain('Backup copies are kept');
      await click(f, 'Delete with the file');
      expect(go.argsOf('TournamentService.DeleteTournament')).toEqual([[path, true]]);
    });

    it('cannot delete a missing file, and shows an empty trash', async () => {
      go.on('TournamentService.ListTrashedTournaments', [{ ...trashed[0], exists: false }]);
      const f = await render(Home);
      await click(f, 'Trash · 1');
      expect(text(f)).toContain('File not found');
      await click(f, 'Delete…');
      expect((find(f, '.confirm-delete input[type=checkbox]') as HTMLInputElement).disabled).toBe(true);
      await click(f, 'Cancel');
      expect(buttons(f, '.confirm-delete').length).toBe(0);
      go.on('TournamentService.ListTrashedTournaments', []);
      await click(f, 'Restore');
      expect(text(f)).toContain('The trash is empty.');
    });

    it('runs every button of the trash', async () => {
      go.on('TournamentService.ListTrashedTournaments', trashed);
      const f = await render(Home);
      await click(f, 'Trash · 1');
      await clickAll(f, ['Back to recent']);
    });
  });
});
