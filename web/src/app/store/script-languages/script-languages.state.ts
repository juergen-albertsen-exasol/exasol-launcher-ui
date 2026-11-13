import { Injectable, inject } from '@angular/core';
import { Action, Selector, State, StateContext } from '@ngxs/store';
import { patch } from '@ngxs/store/operators';
import { EMPTY, Observable, catchError, tap } from 'rxjs';
import { ScriptLanguages } from '@exa/services/local-api/local-api.models';
import { LocalApiService, problemDetail } from '@exa/services/local-api/local-api.service';
import { LoadScriptLanguagesAction } from './script-languages.actions';

export interface ScriptLanguagesStateModel {
  scriptLanguages: { [name: string]: ScriptLanguages };
  /** The API's message when it reports no script languages, as for cloud deployments. */
  loadErrors: { [name: string]: string | undefined };
}

@State<ScriptLanguagesStateModel>({
  name: 'scriptLanguages',
  defaults: { scriptLanguages: {}, loadErrors: {} },
})
@Injectable()
export class ScriptLanguagesState {
  private readonly api = inject(LocalApiService);

  @Selector()
  static scriptLanguages(state: ScriptLanguagesStateModel): ScriptLanguagesStateModel['scriptLanguages'] {
    return state.scriptLanguages;
  }

  @Selector()
  static loadErrors(state: ScriptLanguagesStateModel): ScriptLanguagesStateModel['loadErrors'] {
    return state.loadErrors;
  }

  @Action(LoadScriptLanguagesAction)
  load(ctx: StateContext<ScriptLanguagesStateModel>, { name }: LoadScriptLanguagesAction): Observable<unknown> {
    return this.api.listScriptLanguages(name).pipe(
      tap(scriptLanguages =>
        ctx.setState(
          patch<ScriptLanguagesStateModel>({
            scriptLanguages: patch({ [name]: scriptLanguages }),
            loadErrors: patch({ [name]: undefined }),
          })
        )
      ),
      catchError(error => {
        ctx.setState(patch<ScriptLanguagesStateModel>({ loadErrors: patch({ [name]: problemDetail(error) }) }));
        return EMPTY;
      })
    );
  }
}
