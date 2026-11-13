import { Injectable, inject } from '@angular/core';
import { Action, Selector, State, StateContext } from '@ngxs/store';
import { EMPTY, catchError, tap } from 'rxjs';
import { ErrorNotificationsAction } from '@exasol/ui-common';
import { Operation, OperationStatus } from '@exa/services/local-api/local-api.models';
import { LocalApiService, problemDetail } from '@exa/services/local-api/local-api.service';
import { ACTION, operationSubject } from './operation-labels';
import { LoadOperationsAction, RunOperationAction } from './operations.actions';

export interface OperationsStateModel {
  operations: Operation[];
}

const isRunning = (operation: Operation) => operation.status === OperationStatus.RUNNING;

@State<OperationsStateModel>({
  name: 'operations',
  defaults: { operations: [] },
})
@Injectable()
export class OperationsState {
  private readonly api = inject(LocalApiService);

  @Selector()
  static operations(state: OperationsStateModel): Operation[] {
    return state.operations;
  }

  @Selector()
  static running(state: OperationsStateModel): Operation[] {
    return state.operations.filter(isRunning);
  }

  @Selector()
  static finished(state: OperationsStateModel): Operation[] {
    return state.operations.filter(operation => !isRunning(operation));
  }

  @Action(LoadOperationsAction)
  load(ctx: StateContext<OperationsStateModel>) {
    return this.api.listOperations().pipe(
      tap(operations => ctx.patchState({ operations })),
      catchError(() => EMPTY)
    );
  }

  @Action(RunOperationAction)
  run(ctx: StateContext<OperationsStateModel>, { deployment, kind, params }: RunOperationAction) {
    const request = params ? this.api.runOperation(deployment, kind, params) : this.api.runOperation(deployment, kind);
    return request.pipe(
      tap(operation => ctx.patchState({ operations: [operation, ...ctx.getState().operations] })),
      catchError(error => {
        const target = operationSubject(deployment, kind, params?.sidecar ?? params?.script_language);
        ctx.dispatch(new ErrorNotificationsAction(`Cannot ${ACTION[kind]} ${target}: ${problemDetail(error)}`));
        return EMPTY;
      })
    );
  }
}

/** Polls the operations list, often while operations run and rarely otherwise. */
export const OPERATIONS_POLL_MS = { running: 2000, idle: 10000 } as const;
