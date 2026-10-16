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
  },
  { path: '**', redirectTo: '' },
];
