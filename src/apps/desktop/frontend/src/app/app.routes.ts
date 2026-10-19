import { Routes } from '@angular/router';

export const routes: Routes = [
  {
    path: 'display',
    loadComponent: () =>
      import('./display-screen/display-screen').then((m) => m.DisplayScreen),
  },
  {
    path: '',
    loadComponent: () => import('./shell/shell').then((m) => m.Shell),
    children: [
      { path: '', loadComponent: () => import('./home/home').then((m) => m.Home) },
      {
        path: 'presets',
        loadComponent: () => import('./presets/presets').then((m) => m.PresetsPage),
      },
      {
        path: 'presets/format/:id',
        loadComponent: () => import('./presets/format-preset').then((m) => m.FormatPresetPage),
      },
      {
        path: 'settings',
        loadComponent: () => import('./settings/settings').then((m) => m.SettingsPage),
      },
      {
        path: 'tournament',
        loadComponent: () =>
          import('./tournament-layout/tournament-layout').then((m) => m.TournamentLayout),
        children: [
          { path: '', pathMatch: 'full', redirectTo: 'info' },
          {
            path: 'info',
            loadComponent: () =>
              import('./tournament-info/tournament-info').then((m) => m.TournamentInfo),
          },
          {
            path: 'entrants',
            loadComponent: () => import('./entrants/entrants').then((m) => m.Entrants),
          },
          {
            path: 'format',
            loadComponent: () => import('./format/format').then((m) => m.Format),
          },
          {
            path: 'draw',
            loadComponent: () => import('./draw/draw').then((m) => m.Draw),
          },
          {
            path: 'schedule',
            loadComponent: () => import('./schedule/schedule').then((m) => m.Schedule),
          },
          {
            path: 'control-desk',
            loadComponent: () => import('./control-desk/control-desk').then((m) => m.ControlDesk),
          },
          {
            path: 'stages',
            loadComponent: () => import('./phases/phases').then((m) => m.Phases),
          },
          {
            path: 'results',
            loadComponent: () => import('./results/results').then((m) => m.Results),
          },
          {
            path: 'check-in',
            loadComponent: () => import('./check-in/check-in').then((m) => m.CheckIn),
          },
        ],
      },
    ],
  },
  { path: '**', redirectTo: '' },
];
