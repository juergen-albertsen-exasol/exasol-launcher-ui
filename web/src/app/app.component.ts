import { Component, OnInit, inject } from '@angular/core';
import { RouterModule } from '@angular/router';
import { Store } from '@ngxs/store';
import { finalize } from 'rxjs';
import { LayoutStateService, NotificationsService, ThemeService } from '@exasol/ui-common';
import { SessionExpiredComponent } from './components/session-expired/session-expired.component';
import { SessionService } from './services/session/session.service';
import { LoadDeploymentsAction } from './store/deployments/deployments.actions';
import { OperationsPollingService } from './store/operations/operations-polling.service';

@Component({
  selector: 'exa-root',
  templateUrl: './app.component.html',
  imports: [RouterModule, SessionExpiredComponent],
})
export class AppComponent implements OnInit {
  private readonly notificationsService = inject(NotificationsService);
  private readonly themeService = inject(ThemeService);
  private readonly store = inject(Store);
  private readonly layoutState = inject(LayoutStateService);
  private readonly operationsPolling = inject(OperationsPollingService);
  readonly sessionExpired = inject(SessionService).expired;

  constructor() {
    this.notificationsService.listen();
  }

  ngOnInit(): void {
    this.operationsPolling.start();
    this.themeService.enabled.next(true);
    if (!this.themeService.currentTheme && window.matchMedia?.('(prefers-color-scheme: dark)').matches) {
      this.themeService.setTheme(this.themeService.availableThemes[0]);
    }
    this.store
      .dispatch(new LoadDeploymentsAction())
      .pipe(finalize(() => this.layoutState.loading.set(false)))
      .subscribe();
  }
}
