import { Component, DestroyRef, inject } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { ControlDeskView, PhasesView, ScheduleView, Settings, TournamentView } from '@bindings/apps/desktop';

import { FakeBackend } from '../../testing/backend';
import { render } from '../../testing/render';
import beforeDesk from '../../testing/fixtures/before-desk.json';
import beforeSchedule from '../../testing/fixtures/before-schedule.json';
import beforeTournament from '../../testing/fixtures/before-tournament.json';
import finishedDesk from '../../testing/fixtures/finished-desk.json';
import finishedPhases from '../../testing/fixtures/finished-phases.json';
import finishedTournament from '../../testing/fixtures/finished-tournament.json';
import progressDesk from '../../testing/fixtures/progress-desk.json';
import progressPhases from '../../testing/fixtures/progress-phases.json';
import progressSchedule from '../../testing/fixtures/progress-schedule.json';
import progressTournament from '../../testing/fixtures/progress-tournament.json';
import settings from '../../testing/fixtures/settings.json';
import { SNAPSHOT_DELAY_MS, SnapshotWriter } from './snapshot-writer';
import { esc } from './snapshot-html';

/** The main window: it starts the writer, as the shell does. */
@Component({ template: '' })
class Host {
  constructor() {
    inject(SnapshotWriter).start(inject(DestroyRef));
  }
}

const WRITE = 'TournamentService.WriteSnapshot';

function fakeGo(tournament: unknown, phases: unknown, desk: unknown, schedule: unknown, showBibs = false): FakeBackend {
  return new FakeBackend()
    .on('SettingsService.GetSettings', { ...settings, showBibs } as unknown as Settings)
    .on('TournamentService.GetTournamentView', tournament as TournamentView)
    .on('TournamentService.GetPhasesView', phases as PhasesView)
    .on('TournamentService.GetControlDeskView', desk as ControlDeskView)
    .on('TournamentService.GetScheduleView', schedule as ScheduleView);
}

/** Writes now and returns what Go was given. */
async function write(go: FakeBackend): Promise<{ version: number; html: string }> {
  const before = go.argsOf(WRITE).length;
  await TestBed.inject(SnapshotWriter).write();
  const calls = go.argsOf(WRITE);
  expect(calls.length).toBe(before + 1);
  const [version, html] = calls[calls.length - 1] as [number, string];
  return { version, html };
}

describe('SnapshotWriter', () => {
  it('writes pools, courts, pending scores and the matches left', async () => {
    const go = fakeGo(progressTournament, progressPhases, progressDesk, progressSchedule);
    await render(Host, { lang: 'fr' });
    const { version, html } = await write(go);
    expect(version).toBe(progressTournament.version);
    expect(html.startsWith('<!doctype html>')).toBe(true);
    expect(html).toContain('<html lang="fr">');
    expect(html).toContain(`<h1>${esc(progressTournament.name)}</h1>`);
    expect(html).toContain(esc("Tournoi d'automne.openshuttle"));
    expect(html).toContain('Terrains');
    expect(html).toContain('Terrain 2');
    expect(html).toContain('Scores en attente');
    expect(html).toContain('Poule A');
    expect(html).toContain('Victor Lambert / Kevin André');
    expect(html).toContain('(Plumes du Lac)');
    expect(html).toContain('Pas encore rempli.');
    const left = progressSchedule.slots.filter((s) => s.state === 'planned').length;
    expect(html).toContain(esc(`${left} matchs restants, dans l'ordre de l'échéancier`));
    // Bibs stay hidden unless the settings show them.
    expect(html).not.toContain('#15 Victor Lambert');
  });

  it('shows bibs when the settings do, and the places of a finished bracket', async () => {
    const empty = { ...progressSchedule, slots: [] };
    const go = fakeGo(finishedTournament, finishedPhases, finishedDesk, empty, true);
    await render(Host, { lang: 'en' });
    const { html } = await write(go);
    expect(html).toContain('Places');
    expect(html).toContain('Nathan Brun / Simon Fournier');
    expect(html).toMatch(/#\d+ Nathan Brun/);
    expect(html).toContain('No match left to play.');
  });

  it('says when a category is not drawn yet', async () => {
    const go = fakeGo(beforeTournament, { categories: [{ category: 'MD', slot: 1, started: false, stages: [] }] }, beforeDesk, beforeSchedule);
    await render(Host, { lang: 'en' });
    const { html } = await write(go);
    expect(html).toContain('The draw is not confirmed yet.');
    expect(html).not.toContain('Pending scores');
  });

  it('escapes the names', async () => {
    const go = fakeGo({ ...progressTournament, name: '<b>Open & co</b>' }, progressPhases, progressDesk, progressSchedule);
    await render(Host, { lang: 'en' });
    const { html } = await write(go);
    expect(html).toContain('<title>&lt;b&gt;Open &amp; co&lt;/b&gt;</title>');
    expect(html).not.toContain('<b>Open');
  });

  it('writes nothing without a tournament, and ignores a failure', async () => {
    const go = fakeGo({ ...progressTournament, open: false }, progressPhases, progressDesk, progressSchedule);
    await render(Host);
    await TestBed.inject(SnapshotWriter).write();
    expect(go.called(WRITE)).toBe(false);

    go.on('TournamentService.GetTournamentView', progressTournament).on(WRITE, () => {
      throw new Error('WRITE_FAILED');
    });
    await expect(TestBed.inject(SnapshotWriter).write()).resolves.toBeUndefined();
  });

  it('writes once a burst of actions settles', async () => {
    const go = fakeGo(progressTournament, progressPhases, progressDesk, progressSchedule);
    await render(Host);
    go.emit('tournament.updated', 101);
    go.emit('tournament.updated', 102);
    expect(go.called(WRITE)).toBe(false);
    await new Promise((r) => setTimeout(r, SNAPSHOT_DELAY_MS + 100));
    expect(go.argsOf(WRITE).length).toBe(1);
    TestBed.resetTestingModule();
    go.emit('tournament.updated', 103);
    await new Promise((r) => setTimeout(r, SNAPSHOT_DELAY_MS + 100));
    expect(go.argsOf(WRITE).length).toBe(1);
  });
});
