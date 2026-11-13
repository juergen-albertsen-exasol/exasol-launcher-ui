import { Injectable, inject } from '@angular/core';
import { Action, Selector, State, StateContext } from '@ngxs/store';
import { patch } from '@ngxs/store/operators';
import { EMPTY, Observable, catchError, of, tap } from 'rxjs';
import { DeploymentDetails, DeploymentSummary, PingResult, PingStatus } from '@exa/services/local-api/local-api.models';
import { LocalApiService, problemDetail } from '@exa/services/local-api/local-api.service';
import {
  DeploymentRemovedAction,
  LoadDeploymentDetailsAction,
  LoadDeploymentsAction,
  PingDeploymentAction,
  SelectDeploymentAction,
} from './deployments.actions';
import { DeploymentsStateModel } from './deployments.state.models';

const without = <T>(map: { [key: string]: T }, key: string): { [key: string]: T } => {
  const copy = { ...map };
  delete copy[key];
  return copy;
};

const defaults: DeploymentsStateModel = {
  details: {},
  detailsErrors: {},
  pings: {},
  pingsInProgress: {},
};

@State<DeploymentsStateModel>({
  name: 'deployments',
  defaults,
})
@Injectable()
export class DeploymentsState {
  private readonly api = inject(LocalApiService);

  @Selector()
  static deployments(state: DeploymentsStateModel): DeploymentSummary[] | undefined {
    return state.deployments;
  }

  @Selector()
  static listError(state: DeploymentsStateModel): string | undefined {
    return state.listError;
  }

  @Selector()
  static selectedName(state: DeploymentsStateModel): string | undefined {
    return state.selectedName;
  }

  @Selector()
  static selectedSummary(state: DeploymentsStateModel): DeploymentSummary | undefined {
    return state.deployments?.find(deployment => deployment.name === state.selectedName);
  }

  @Selector()
  static selectedDetails(state: DeploymentsStateModel): DeploymentDetails | undefined {
    return state.selectedName ? state.details[state.selectedName] : undefined;
  }

  @Selector()
  static selectedDetailsError(state: DeploymentsStateModel): string | undefined {
    return state.selectedName ? state.detailsErrors[state.selectedName] : undefined;
  }

  @Selector()
  static selectedPing(state: DeploymentsStateModel): PingResult | undefined {
    return state.selectedName ? state.pings[state.selectedName] : undefined;
  }

  @Selector()
  static selectedPingInProgress(state: DeploymentsStateModel): boolean {
    return !!state.selectedName && !!state.pingsInProgress[state.selectedName];
  }

  @Action(LoadDeploymentsAction)
  loadDeployments(ctx: StateContext<DeploymentsStateModel>, { refresh }: LoadDeploymentsAction) {
    return this.api.listDeployments(refresh).pipe(
      tap(deployments => {
        ctx.patchState({ deployments, listError: undefined });
        const selected = ctx.getState().selectedName;
        if (refresh && selected) {
          ctx.dispatch(new SelectDeploymentAction(selected));
        }
      }),
      catchError(error => {
        ctx.patchState({ listError: problemDetail(error) });
        return EMPTY;
      })
    );
  }

  // The listing takes the launcher seconds, so a removed deployment leaves it at once.
  @Action(DeploymentRemovedAction)
  removed(ctx: StateContext<DeploymentsStateModel>, { name }: DeploymentRemovedAction) {
    const { deployments, selectedName } = ctx.getState();
    ctx.patchState({
      deployments: deployments?.filter(deployment => deployment.name !== name),
      selectedName: selectedName === name ? undefined : selectedName,
    });
    return ctx.dispatch(new LoadDeploymentsAction(true));
  }

  @Action(SelectDeploymentAction)
  selectDeployment(ctx: StateContext<DeploymentsStateModel>, { name }: SelectDeploymentAction) {
    ctx.patchState({ selectedName: name });
    return ctx.dispatch([new LoadDeploymentDetailsAction(name), new PingDeploymentAction(name)]);
  }

  @Action(LoadDeploymentDetailsAction)
  loadDetails(ctx: StateContext<DeploymentsStateModel>, { name }: LoadDeploymentDetailsAction): Observable<unknown> {
    return this.api.getDeployment(name).pipe(
      tap(details => {
        const { details: allDetails, detailsErrors } = ctx.getState();
        ctx.patchState({ details: { ...allDetails, [name]: details }, detailsErrors: without(detailsErrors, name) });
      }),
      catchError(error => {
        ctx.setState(patch<DeploymentsStateModel>({ detailsErrors: patch({ [name]: problemDetail(error) }) }));
        return EMPTY;
      })
    );
  }

  @Action(PingDeploymentAction)
  ping(ctx: StateContext<DeploymentsStateModel>, { name }: PingDeploymentAction): Observable<unknown> {
    ctx.setState(patch<DeploymentsStateModel>({ pingsInProgress: patch({ [name]: true }) }));

    return this.api.pingDeployment(name).pipe(
      catchError(error => of<PingResult>({ status: PingStatus.UNREACHABLE, error: problemDetail(error) })),
      tap(result =>
        ctx.setState(
          patch<DeploymentsStateModel>({
            pings: patch({ [name]: result }),
            pingsInProgress: patch({ [name]: false }),
          })
        )
      )
    );
  }
}
