import { Component, OnInit, inject } from '@angular/core';
import { RouterModule } from '@angular/router';
import { LayoutComponent, NotificationsService, ThemeService } from '@exasol/ui-common';

@Component({
  selector: 'exa-root',
  templateUrl: './app.component.html',
  styleUrls: ['./app.component.scss'],
  imports: [LayoutComponent, RouterModule],
})
export class AppComponent implements OnInit {
  private readonly notificationsService = inject(NotificationsService);
  private readonly themeService = inject(ThemeService);

  constructor() {
    this.notificationsService.listen();
  }

  ngOnInit(): void {
    this.themeService.enabled.next(true);
    if (!this.themeService.currentTheme && window.matchMedia?.('(prefers-color-scheme: dark)').matches) {
      this.themeService.setTheme(this.themeService.availableThemes[0]);
    }
  }
}
