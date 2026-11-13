import { Injectable, inject } from '@angular/core';
import { Action, Selector, State, StateContext } from '@ngxs/store';
import { patch } from '@ngxs/store/operators';
import { EMPTY, Observable, catchError, tap } from 'rxjs';
import { McpClient, McpInstructions, Sidecar } from '@exa/services/local-api/local-api.models';
import { LocalApiService, problemDetail } from '@exa/services/local-api/local-api.service';
import { LoadMcpClientsAction, LoadMcpInstructionsAction, LoadSidecarsAction } from './sidecars.actions';

export interface SidecarsStateModel {
  sidecars: { [name: string]: Sidecar[] };
  /** The launcher's message when it reports no sidecars, as for cloud deployments. */
  loadErrors: { [name: string]: string | undefined };
  mcpClients: { [name: string]: McpClient[] };
  /** The instructions for the client last requested on each deployment. */
  mcpInstructions: { [name: string]: McpInstructions | undefined };
  mcpErrors: { [name: string]: string | undefined };
}

@State<SidecarsStateModel>({
  name: 'sidecars',
  defaults: { sidecars: {}, loadErrors: {}, mcpClients: {}, mcpInstructions: {}, mcpErrors: {} },
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

  @Selector()
  static mcpClients(state: SidecarsStateModel): SidecarsStateModel['mcpClients'] {
    return state.mcpClients;
  }

  @Selector()
  static mcpInstructions(state: SidecarsStateModel): SidecarsStateModel['mcpInstructions'] {
    return state.mcpInstructions;
  }

  @Selector()
  static mcpErrors(state: SidecarsStateModel): SidecarsStateModel['mcpErrors'] {
    return state.mcpErrors;
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

  @Action(LoadMcpClientsAction)
  loadMcpClients(ctx: StateContext<SidecarsStateModel>, { name }: LoadMcpClientsAction): Observable<unknown> {
    return this.api.listMcpClients(name).pipe(
      tap(clients => ctx.setState(patch<SidecarsStateModel>({ mcpClients: patch({ [name]: clients }) }))),
      catchError(error => {
        ctx.setState(patch<SidecarsStateModel>({ mcpErrors: patch({ [name]: problemDetail(error) }) }));
        return EMPTY;
      })
    );
  }

  @Action(LoadMcpInstructionsAction, { cancelUncompleted: true })
  loadMcpInstructions(
    ctx: StateContext<SidecarsStateModel>,
    { name, client }: LoadMcpInstructionsAction
  ): Observable<unknown> {
    return this.api.getMcpInstructions(name, client).pipe(
      tap(instructions =>
        ctx.setState(
          patch<SidecarsStateModel>({
            mcpInstructions: patch({ [name]: instructions }),
            mcpErrors: patch({ [name]: undefined }),
          })
        )
      ),
      catchError(error => {
        ctx.setState(
          patch<SidecarsStateModel>({
            mcpInstructions: patch({ [name]: undefined }),
            mcpErrors: patch({ [name]: problemDetail(error) }),
          })
        );
        return EMPTY;
      })
    );
  }
}
