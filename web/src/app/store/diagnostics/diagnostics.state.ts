import { Injectable, inject } from '@angular/core';
import { Action, Selector, State, StateContext } from '@ngxs/store';
import { patch } from '@ngxs/store/operators';
import { EMPTY, Observable, catchError, finalize, tap } from 'rxjs';
import { ErrorNotificationsAction, SuccessNotificationsAction } from '@exasol/ui-common';
import { DiagnosticReport, LocalRuntime } from '@exa/services/local-api/local-api.models';
import { LocalApiService, problemDetail } from '@exa/services/local-api/local-api.service';
import { LoadDiagnosticReportAction, LoadLocalRuntimeAction, UnlockDeploymentAction } from './diagnostics.actions';

export interface DiagnosticsStateModel {
  localRuntimes: { [name: string]: LocalRuntime };
  localRuntimeErrors: { [name: string]: string | undefined };
  /** Deployments whose local runtime is being checked; the launcher takes up to 30 seconds. */
  checking: { [name: string]: boolean };
  reports: { [name: string]: DiagnosticReport };
  reportErrors: { [name: string]: string | undefined };
}

@State<DiagnosticsStateModel>({
  name: 'diagnostics',
  defaults: { localRuntimes: {}, localRuntimeErrors: {}, checking: {}, reports: {}, reportErrors: {} },
})
@Injectable()
export class DiagnosticsState {
  private readonly api = inject(LocalApiService);

  @Selector()
  static localRuntimes(state: DiagnosticsStateModel): DiagnosticsStateModel['localRuntimes'] {
    return state.localRuntimes;
  }

  @Selector()
  static localRuntimeErrors(state: DiagnosticsStateModel): DiagnosticsStateModel['localRuntimeErrors'] {
    return state.localRuntimeErrors;
  }

  @Selector()
  static checking(state: DiagnosticsStateModel): DiagnosticsStateModel['checking'] {
    return state.checking;
  }

  @Selector()
  static reports(state: DiagnosticsStateModel): DiagnosticsStateModel['reports'] {
    return state.reports;
  }

  @Selector()
  static reportErrors(state: DiagnosticsStateModel): DiagnosticsStateModel['reportErrors'] {
    return state.reportErrors;
  }

  @Action(LoadLocalRuntimeAction)
  loadLocalRuntime(ctx: StateContext<DiagnosticsStateModel>, { name }: LoadLocalRuntimeAction): Observable<unknown> {
    ctx.setState(patch<DiagnosticsStateModel>({ checking: patch({ [name]: true }) }));
    return this.api.getLocalRuntime(name).pipe(
      tap(runtime =>
        ctx.setState(
          patch<DiagnosticsStateModel>({
            localRuntimes: patch({ [name]: runtime }),
            localRuntimeErrors: patch({ [name]: undefined }),
          })
        )
      ),
      catchError(error => {
        ctx.setState(patch<DiagnosticsStateModel>({ localRuntimeErrors: patch({ [name]: problemDetail(error) }) }));
        return EMPTY;
      }),
      finalize(() => ctx.setState(patch<DiagnosticsStateModel>({ checking: patch({ [name]: false }) })))
    );
  }

  @Action(UnlockDeploymentAction)
  unlock(ctx: StateContext<DiagnosticsStateModel>, { name }: UnlockDeploymentAction): Observable<unknown> {
    return this.api.unlock(name).pipe(
      tap(() => ctx.dispatch(new SuccessNotificationsAction(`Unlocked the folder of ${name}.`))),
      catchError(error => {
        ctx.dispatch(new ErrorNotificationsAction(`Cannot unlock the folder of ${name}: ${problemDetail(error)}`));
        return EMPTY;
      })
    );
  }

  @Action(LoadDiagnosticReportAction)
  loadReport(ctx: StateContext<DiagnosticsStateModel>, { name }: LoadDiagnosticReportAction): Observable<unknown> {
    return this.api.getDiagnosticReport(name).pipe(
      tap(report =>
        ctx.setState(
          patch<DiagnosticsStateModel>({
            reports: patch({ [name]: report }),
            reportErrors: patch({ [name]: undefined }),
          })
        )
      ),
      catchError(error => {
        ctx.setState(patch<DiagnosticsStateModel>({ reportErrors: patch({ [name]: problemDetail(error) }) }));
        return EMPTY;
      })
    );
  }
}
