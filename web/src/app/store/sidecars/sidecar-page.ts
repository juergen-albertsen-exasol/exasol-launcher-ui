import { NgZone, Signal, computed, effect, inject, untracked } from '@angular/core';
import { Store } from '@ngxs/store';
import { Operation, OperationStatus, Sidecar, SidecarHealth } from '@exa/services/local-api/local-api.models';
import { LoadDeploymentsAction, SelectDeploymentAction } from '@exa/store/deployments/deployments.actions';
import { DeploymentsState } from '@exa/store/deployments/deployments.state';
import { OperationsState } from '@exa/store/operations/operations.state';
import { LoadSidecarsAction } from './sidecars.actions';
import { SidecarsState } from './sidecars.state';

export const STARTING_REFRESH_MS = 3000;

const RUNNING_STATUSES = ['running', 'database_ready', 'database_connection_failed'];

/** Shows how a sidecar endpoint is reached: its URL without the scheme, or its address. */
export const endpointText = (endpoint: Sidecar['endpoints'][number]): string =>
  endpoint.url ? endpoint.url.replace(/^[a-z]+:\/\//, '').replace(/\/$/, '') : `${endpoint.ip}:${endpoint.port}`;

/**
 * Loads the sidecars of the named deployment when a page opens, when an
 * operation on the deployment ends, and repeatedly while a sidecar starts.
 * Call it from a component constructor.
 */
export const followSidecars = (name: Signal<string>) => {
  const store = inject(Store);
  const zone = inject(NgZone);

  const allSidecars = store.selectSignal(SidecarsState.sidecars);
  const loadErrors = store.selectSignal(SidecarsState.loadErrors);
  const operations = store.selectSignal(OperationsState.operations);
  const summary = store.selectSignal(DeploymentsState.selectedSummary);

  const sidecars = computed<Sidecar[] | undefined>(() => allSidecars()[name()]);
  const loadError = computed(() => loadErrors()[name()]);
  const latest = computed(() => operations().find(operation => operation.deployment === name()));
  const running = computed(() => {
    const operation = latest();
    return operation?.status === OperationStatus.RUNNING ? operation : undefined;
  });
  const failed = computed(() => {
    const operation = latest();
    return operation?.status === OperationStatus.FAILED ? operation : undefined;
  });
  const deploymentStopped = computed(() => {
    const status = summary()?.status;
    return !!status && !RUNNING_STATUSES.includes(status);
  });

  effect(() => {
    const deployment = name();
    untracked(() => store.dispatch([new SelectDeploymentAction(deployment), new LoadSidecarsAction(deployment)]));
  });

  let previous: Operation | undefined;
  effect(() => {
    const operation = running();
    const deployment = untracked(name);
    if (previous?.deployment === deployment && !operation) {
      untracked(() => store.dispatch([new LoadDeploymentsAction(true), new LoadSidecarsAction(deployment)]));
    }
    previous = operation;
  });

  // Each listing is a new array, so the effect runs again after every reload.
  // The timer runs outside Angular's zone, so pages can still become stable.
  effect(onCleanup => {
    const starting = sidecars()?.some(sidecar => sidecar.health === SidecarHealth.STARTING);
    if (!starting || running()) {
      return;
    }
    const deployment = untracked(name);
    const timer = zone.runOutsideAngular(() =>
      setTimeout(() => zone.run(() => store.dispatch(new LoadSidecarsAction(deployment))), STARTING_REFRESH_MS)
    );
    onCleanup(() => clearTimeout(timer));
  });

  return { sidecars, loadError, running, failed, deploymentStopped };
};
