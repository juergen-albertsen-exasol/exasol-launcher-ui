import { Component, effect, inject, input } from '@angular/core';
import { Store } from '@ngxs/store';
import { ExaButtonComponent } from '@exasol/ui-common';
import { FolderLinkComponent } from '@exa/components/folder-link/folder-link.component';
import { DEPLOYMENT_READY_STATUS, PingStatus } from '@exa/services/local-api/local-api.models';
import { PingDeploymentAction, SelectDeploymentAction } from '@exa/store/deployments/deployments.actions';
import { DeploymentsState } from '@exa/store/deployments/deployments.state';

@Component({
  selector: 'exa-page-overview',
  templateUrl: './page-overview.component.html',
  styleUrls: ['./page-overview.component.scss'],
  imports: [ExaButtonComponent, FolderLinkComponent],
})
export class PageOverviewComponent {
  private readonly store = inject(Store);

  readonly name = input.required<string>();

  readonly pingStatus = PingStatus;
  readonly readyStatus = DEPLOYMENT_READY_STATUS;
  readonly summary = this.store.selectSignal(DeploymentsState.selectedSummary);
  readonly details = this.store.selectSignal(DeploymentsState.selectedDetails);
  readonly detailsError = this.store.selectSignal(DeploymentsState.selectedDetailsError);
  readonly ping = this.store.selectSignal(DeploymentsState.selectedPing);
  readonly pingInProgress = this.store.selectSignal(DeploymentsState.selectedPingInProgress);

  constructor() {
    effect(() => this.store.dispatch(new SelectDeploymentAction(this.name())));
  }

  checkAgain(): void {
    this.store.dispatch(new PingDeploymentAction(this.name()));
  }
}
