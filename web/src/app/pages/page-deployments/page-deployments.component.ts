import { Component, inject } from '@angular/core';
import { RouterLink } from '@angular/router';
import { Store } from '@ngxs/store';
import { FolderLinkComponent } from '@exa/components/folder-link/folder-link.component';
import { overviewPath } from '@exa/routes';
import { DEPLOYMENT_READY_STATUS } from '@exa/services/local-api/local-api.models';
import { DeploymentsState } from '@exa/store/deployments/deployments.state';

@Component({
  selector: 'exa-page-deployments',
  templateUrl: './page-deployments.component.html',
  styleUrls: ['./page-deployments.component.scss'],
  imports: [RouterLink, FolderLinkComponent],
})
export class PageDeploymentsComponent {
  private readonly store = inject(Store);

  readonly overviewPath = overviewPath;
  readonly readyStatus = DEPLOYMENT_READY_STATUS;
  readonly deployments = this.store.selectSignal(DeploymentsState.deployments);
  readonly listError = this.store.selectSignal(DeploymentsState.listError);
}
