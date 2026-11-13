import { Injectable, inject } from '@angular/core';
import { Action, Selector, State, StateContext } from '@ngxs/store';
import { patch } from '@ngxs/store/operators';
import { EMPTY, Observable, catchError, tap } from 'rxjs';
import { Configuration, PresetKind } from '@exa/services/local-api/local-api.models';
import { LocalApiService, problemDetail } from '@exa/services/local-api/local-api.service';
import {
  ChangesAppliedAction,
  LoadConfigurationAction,
  ResetOptionsAction,
  SaveOptionsAction,
} from './settings.actions';

export interface SaveError {
  preset: PresetKind;
  message: string;
}

export interface SettingsStateModel {
  configurations: { [name: string]: Configuration };
  loadErrors: { [name: string]: string | undefined };
  saving: { [name: string]: PresetKind | undefined };
  saveErrors: { [name: string]: SaveError | undefined };
  /** Whether the user saved or reset options since an operation last finished on the deployment. */
  unapplied: { [name: string]: boolean };
}

@State<SettingsStateModel>({
  name: 'settings',
  defaults: { configurations: {}, loadErrors: {}, saving: {}, saveErrors: {}, unapplied: {} },
})
@Injectable()
export class SettingsState {
  private readonly api = inject(LocalApiService);

  @Selector()
  static configurations(state: SettingsStateModel): SettingsStateModel['configurations'] {
    return state.configurations;
  }

  @Selector()
  static loadErrors(state: SettingsStateModel): SettingsStateModel['loadErrors'] {
    return state.loadErrors;
  }

  @Selector()
  static saving(state: SettingsStateModel): SettingsStateModel['saving'] {
    return state.saving;
  }

  @Selector()
  static saveErrors(state: SettingsStateModel): SettingsStateModel['saveErrors'] {
    return state.saveErrors;
  }

  @Selector()
  static unapplied(state: SettingsStateModel): SettingsStateModel['unapplied'] {
    return state.unapplied;
  }

  @Action(LoadConfigurationAction)
  load(ctx: StateContext<SettingsStateModel>, { name }: LoadConfigurationAction): Observable<unknown> {
    return this.api.getConfiguration(name).pipe(
      tap(configuration =>
        ctx.setState(
          patch<SettingsStateModel>({
            configurations: patch({ [name]: configuration }),
            loadErrors: patch({ [name]: undefined }),
          })
        )
      ),
      catchError(error => {
        ctx.setState(patch<SettingsStateModel>({ loadErrors: patch({ [name]: problemDetail(error) }) }));
        return EMPTY;
      })
    );
  }

  @Action(SaveOptionsAction)
  save(ctx: StateContext<SettingsStateModel>, { name, preset, options }: SaveOptionsAction): Observable<unknown> {
    return this.change(ctx, name, preset, this.api.setConfiguration(name, options));
  }

  @Action(ResetOptionsAction)
  reset(ctx: StateContext<SettingsStateModel>, { name, preset, options }: ResetOptionsAction): Observable<unknown> {
    return this.change(ctx, name, preset, this.api.resetConfiguration(name, options));
  }

  @Action(ChangesAppliedAction)
  applied(ctx: StateContext<SettingsStateModel>, { name }: ChangesAppliedAction): void {
    ctx.setState(patch<SettingsStateModel>({ unapplied: patch({ [name]: false }) }));
  }

  private change(
    ctx: StateContext<SettingsStateModel>,
    name: string,
    preset: PresetKind,
    request: Observable<Configuration>
  ): Observable<unknown> {
    ctx.setState(
      patch<SettingsStateModel>({ saving: patch({ [name]: preset }), saveErrors: patch({ [name]: undefined }) })
    );
    return request.pipe(
      tap(configuration =>
        ctx.setState(
          patch<SettingsStateModel>({
            configurations: patch({ [name]: configuration }),
            saving: patch({ [name]: undefined }),
            unapplied: patch({ [name]: true }),
          })
        )
      ),
      catchError(error => {
        ctx.setState(
          patch<SettingsStateModel>({
            saving: patch({ [name]: undefined }),
            saveErrors: patch({ [name]: { preset, message: problemDetail(error) } }),
          })
        );
        return EMPTY;
      })
    );
  }
}
