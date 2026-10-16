import { Component } from '@angular/core';
import { TranslocoDirective } from '@jsverse/transloco';

/** Display screen: second window on the hall monitor. Read-only, read from 10 m. */
@Component({
  selector: 'app-display-screen',
  imports: [TranslocoDirective],
  templateUrl: './display-screen.html',
  styleUrl: './display-screen.scss',
})
export class DisplayScreen {}
