import { Component, computed, effect, inject, input, untracked } from '@angular/core';
import { Store } from '@ngxs/store';
import { ExaButtonComponent } from '@exasol/ui-common';
import { FolderLinkComponent } from '@exa/components/folder-link/folder-link.component';
import { OperationProgressComponent } from '@exa/components/operation-progress/operation-progress.component';
import {
  DEPLOYMENT_READY_STATUS,
  Operation,
  OperationKind,
  OperationStatus,
  PingStatus,
} from '@exa/services/local-api/local-api.models';
import {
  LoadDeploymentsAction,
  PingDeploymentAction,
  SelectDeploymentAction,
} from '@exa/store/deployments/deployments.actions';
import { DeploymentsState } from '@exa/store/deployments/deployments.state';
import { RUNNING_VERB } from '@exa/store/operations/operation-labels';
import { RunOperationAction } from '@exa/store/operations/operations.actions';
import { OperationsState } from '@exa/store/operations/operations.state';

const STOPPABLE = ['running', 'database_ready', 'database_connection_failed'];
const RUNNING_STATE: Record<OperationKind, string> = {
  start: 'starting',
  stop: 'stopping',
  deploy: 'deploying',
  destroy: 'destroying',
  destroy_and_remove: 'destroying',
  remove: 'removing',
};

@Component({
  selector: 'exa-page-overview',
  templateUrl: './page-overview.component.html',
  styleUrls: ['./page-overview.component.scss'],
  imports: [ExaButtonComponent, FolderLinkComponent, OperationProgressComponent],
})
export class PageOverviewComponent {
  private readonly store = inject(Store);

  readonly name = input.required<string>();

  readonly pingStatus = PingStatus;
  readonly readyStatus = DEPLOYMENT_READY_STATUS;
  readonly operationKind = OperationKind;
  readonly runningState = RUNNING_STATE;
  readonly operationTitle = RUNNING_VERB;
  readonly summary = this.store.selectSignal(DeploymentsState.selectedSummary);
  readonly details = this.store.selectSignal(DeploymentsState.selectedDetails);
  readonly detailsError = this.store.selectSignal(DeploymentsState.selectedDetailsError);
  readonly ping = this.store.selectSignal(DeploymentsState.selectedPing);
  readonly pingInProgress = this.store.selectSignal(DeploymentsState.selectedPingInProgress);

  private readonly operations = this.store.selectSignal(OperationsState.operations);
  private readonly latest = computed(() => this.operations().find(operation => operation.deployment === this.name()));
  readonly running = computed(() => {
    const latest = this.latest();
    return latest?.status === OperationStatus.RUNNING ? latest : undefined;
  });
  readonly failed = computed(() => {
    const latest = this.latest();
    return latest?.status === OperationStatus.FAILED ? latest : undefined;
  });
  // The listing's status checks the runtime, while `info` reports the stored state.
  readonly canStart = computed(() => !this.running() && this.summary()?.status === 'stopped');
  readonly canStop = computed(() => !this.running() && STOPPABLE.includes(this.summary()?.status ?? ''));

  constructor() {
    effect(() => this.store.dispatch(new SelectDeploymentAction(this.name())));
    this.refreshWhenOperationEnds();
  }

  checkAgain(): void {
    this.store.dispatch(new PingDeploymentAction(this.name()));
  }

  run(kind: OperationKind): void {
    this.store.dispatch(new RunOperationAction(this.name(), kind));
  }

  private refreshWhenOperationEnds(): void {
    let previous: Operation | undefined;
    effect(() => {
      const running = this.running();
      if (previous && !running) {
        untracked(() => this.store.dispatch(new LoadDeploymentsAction(true)));
      }
      previous = running;
    });
  }
}
