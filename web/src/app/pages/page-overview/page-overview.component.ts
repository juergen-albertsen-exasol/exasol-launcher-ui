import { Component, ElementRef, computed, effect, inject, input, signal, untracked, viewChild } from '@angular/core';
import { Store } from '@ngxs/store';
import { ExaButtonComponent } from '@exasol/ui-common';
import { FolderLinkComponent } from '@exa/components/folder-link/folder-link.component';
import {
  DEPLOYMENT_READY_STATUS,
  Operation,
  OperationKind,
  OperationStatus,
  PingStatus,
} from '@exa/services/local-api/local-api.models';
import { OperationEventsService } from '@exa/services/local-api/operation-events.service';
import {
  LoadDeploymentsAction,
  PingDeploymentAction,
  SelectDeploymentAction,
} from '@exa/store/deployments/deployments.actions';
import { DeploymentsState } from '@exa/store/deployments/deployments.state';
import { RUNNING_VERB } from '@exa/store/operations/operation-labels';
import { LoadOperationsAction, RunOperationAction } from '@exa/store/operations/operations.actions';
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
  imports: [ExaButtonComponent, FolderLinkComponent],
})
export class PageOverviewComponent {
  private readonly store = inject(Store);
  private readonly events = inject(OperationEventsService);
  private readonly log = viewChild<ElementRef<HTMLElement>>('log');

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
  readonly lines = signal<string[]>([]);

  constructor() {
    effect(() => this.store.dispatch(new SelectDeploymentAction(this.name())));
    this.followRunningOperation();
    this.refreshWhenOperationEnds();
    effect(() => {
      this.lines();
      const log = this.log()?.nativeElement;
      if (log) {
        log.scrollTop = log.scrollHeight;
      }
    });
  }

  checkAgain(): void {
    this.store.dispatch(new PingDeploymentAction(this.name()));
  }

  run(kind: OperationKind): void {
    this.store.dispatch(new RunOperationAction(this.name(), kind));
  }

  private followRunningOperation(): void {
    effect(onCleanup => {
      const id = this.running()?.id;
      if (!id) {
        return;
      }
      untracked(() => this.lines.set([]));
      const subscription = this.events.follow(id).subscribe(event => {
        if (event.type === 'line') {
          this.lines.update(lines => [...lines, event.line]);
        } else {
          this.store.dispatch(new LoadOperationsAction());
        }
      });
      onCleanup(() => subscription.unsubscribe());
    });
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
