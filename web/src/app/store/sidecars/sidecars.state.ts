import { Injectable, inject } from '@angular/core';
import { Action, Selector, State, StateContext } from '@ngxs/store';
import { patch } from '@ngxs/store/operators';
import { EMPTY, Observable, catchError, tap } from 'rxjs';
import { Sidecar } from '@exa/services/local-api/local-api.models';
import { LocalApiService, problemDetail } from '@exa/services/local-api/local-api.service';
import { LoadSidecarsAction } from './sidecars.actions';

export interface SidecarsStateModel {
  sidecars: { [name: string]: Sidecar[] };
  /** The launcher's message when it reports no sidecars, as for cloud deployments. */
  loadErrors: { [name: string]: string | undefined };
}

@State<SidecarsStateModel>({
  name: 'sidecars',
  defaults: { sidecars: {}, loadErrors: {} },
})
@Injectable()
export class SidecarsState {
  private readonly api = inject(LocalApiService);

  @Selector()
  static sidecars(state: SidecarsStateModel): SidecarsStateModel['sidecars'] {
    return state.sidecars;
  }

  @Selector()
  static loadErrors(state: SidecarsStateModel): SidecarsStateModel['loadErrors'] {
    return state.loadErrors;
  }

  @Action(LoadSidecarsAction)
  load(ctx: StateContext<SidecarsStateModel>, { name }: LoadSidecarsAction): Observable<unknown> {
    return this.api.listSidecars(name).pipe(
      tap(sidecars =>
        ctx.setState(
          patch<SidecarsStateModel>({
            sidecars: patch({ [name]: sidecars }),
            loadErrors: patch({ [name]: undefined }),
          })
        )
      ),
      catchError(error => {
        ctx.setState(patch<SidecarsStateModel>({ loadErrors: patch({ [name]: problemDetail(error) }) }));
        return EMPTY;
      })
    );
  }
}
