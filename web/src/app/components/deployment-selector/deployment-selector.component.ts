import { Component, inject } from '@angular/core';
import { MatSelectChange, MatSelectModule } from '@angular/material/select';
import { MatTooltipModule } from '@angular/material/tooltip';
import { Router } from '@angular/router';
import { Store } from '@ngxs/store';
import { DeploymentPage, deploymentPath } from '@exa/routes';
import { LoadDeploymentsAction } from '@exa/store/deployments/deployments.actions';
import { DeploymentsState } from '@exa/store/deployments/deployments.state';

@Component({
  selector: 'exa-deployment-selector',
  templateUrl: './deployment-selector.component.html',
  styleUrls: ['./deployment-selector.component.scss'],
  imports: [MatSelectModule, MatTooltipModule],
})
export class DeploymentSelectorComponent {
  private readonly store = inject(Store);
  private readonly router = inject(Router);

  readonly deployments = this.store.selectSignal(DeploymentsState.deployments);
  readonly selectedName = this.store.selectSignal(DeploymentsState.selectedName);

  select(change: MatSelectChange): void {
    this.router.navigate(deploymentPath(change.value, this.currentPage()));
  }

  refresh(): void {
    this.store.dispatch(new LoadDeploymentsAction(true));
  }

  private currentPage(): DeploymentPage {
    const segments = this.router.parseUrl(this.router.url).root.children['primary']?.segments ?? [];
    const page = segments[2]?.path;
    return Object.values(DeploymentPage).find(known => known === page) ?? DeploymentPage.OVERVIEW;
  }
}
