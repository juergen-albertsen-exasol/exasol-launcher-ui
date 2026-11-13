import { Component, computed, effect, inject, input, signal, untracked } from '@angular/core';
import { MatDialog } from '@angular/material/dialog';
import { Store } from '@ngxs/store';
import { ConfirmationModalComponent, ExaButtonComponent } from '@exasol/ui-common';
import { OperationProgressComponent } from '@exa/components/operation-progress/operation-progress.component';
import {
  Operation,
  OperationKind,
  OperationParameters,
  OperationStatus,
  ScriptLanguages,
} from '@exa/services/local-api/local-api.models';
import { LoadDeploymentsAction, SelectDeploymentAction } from '@exa/store/deployments/deployments.actions';
import { runningLabel } from '@exa/store/operations/operation-labels';
import { RunOperationAction } from '@exa/store/operations/operations.actions';
import { OperationsState } from '@exa/store/operations/operations.state';
import { LoadScriptLanguagesAction } from '@exa/store/script-languages/script-languages.actions';
import { ScriptLanguagesState } from '@exa/store/script-languages/script-languages.state';

const CUSTOM_KINDS: OperationKind[] = [
  OperationKind.INSTALL_CUSTOM_SCRIPT_LANGUAGE,
  OperationKind.UPDATE_CUSTOM_SCRIPT_LANGUAGE,
];

@Component({
  selector: 'exa-page-script-languages',
  templateUrl: './page-script-languages.component.html',
  styleUrls: ['./page-script-languages.component.scss'],
  imports: [ExaButtonComponent, OperationProgressComponent],
})
export class PageScriptLanguagesComponent {
  private readonly store = inject(Store);
  private readonly dialog = inject(MatDialog);

  readonly name = input.required<string>();

  /** Whether the page's actions restart the database now rather than on its next start. */
  readonly restart = signal(true);
  readonly alias = signal('');
  readonly language = signal('');
  readonly source = signal('');

  private readonly allScriptLanguages = this.store.selectSignal(ScriptLanguagesState.scriptLanguages);
  private readonly loadErrors = this.store.selectSignal(ScriptLanguagesState.loadErrors);
  private readonly operations = this.store.selectSignal(OperationsState.operations);

  readonly scriptLanguages = computed<ScriptLanguages | undefined>(() => this.allScriptLanguages()[this.name()]);
  readonly loadError = computed(() => this.loadErrors()[this.name()]);
  private readonly latest = computed(() => this.operations().find(operation => operation.deployment === this.name()));
  readonly running = computed(() => {
    const operation = this.latest();
    return operation?.status === OperationStatus.RUNNING ? operation : undefined;
  });
  readonly failed = computed(() => {
    const operation = this.latest();
    return operation?.status === OperationStatus.FAILED ? operation : undefined;
  });
  readonly customComplete = computed(() => !!this.alias().trim() && !!this.language().trim() && !!this.source().trim());

  readonly kind = OperationKind;
  readonly runningLabel = runningLabel;

  constructor() {
    effect(() => {
      const deployment = this.name();
      untracked(() =>
        this.store.dispatch([new SelectDeploymentAction(deployment), new LoadScriptLanguagesAction(deployment)])
      );
    });

    let previous: Operation | undefined;
    effect(() => {
      const operation = this.running();
      const deployment = untracked(this.name);
      if (previous?.deployment === deployment && !operation) {
        untracked(() => this.operationEnded(deployment, previous!));
      }
      previous = operation;
    });
  }

  run(kind: OperationKind, scriptLanguage: string): void {
    this.dispatch(kind, { script_language: scriptLanguage });
  }

  remove(alias: string): void {
    const deployment = this.name();
    const restart = this.restart();
    const effectText = restart
      ? 'The database restarts to apply the change.'
      : `The change applies on the next start of ${deployment}.`;
    ConfirmationModalComponent.open(this.dialog, {
      title: `Remove ${alias}?`,
      content: `This removes the script language container ${alias} from ${deployment}. ${effectText}`,
      focusOnCancel: true,
    })
      .afterClosed()
      .subscribe(confirmed => {
        if (confirmed) {
          this.store.dispatch(
            new RunOperationAction(deployment, OperationKind.REMOVE_SCRIPT_LANGUAGE, {
              script_language: alias,
              restart,
            })
          );
        }
      });
  }

  runCustom(kind: OperationKind): void {
    if (this.customComplete()) {
      this.dispatch(kind, {
        script_language: this.alias().trim(),
        language: this.language().trim(),
        source: this.source().trim(),
      });
    }
  }

  private dispatch(kind: OperationKind, params: OperationParameters): void {
    this.store.dispatch(new RunOperationAction(this.name(), kind, { ...params, restart: this.restart() }));
  }

  private operationEnded(deployment: string, ended: Operation): void {
    this.store.dispatch([new LoadDeploymentsAction(true), new LoadScriptLanguagesAction(deployment)]);
    const outcome = this.operations().find(operation => operation.id === ended.id);
    if (CUSTOM_KINDS.includes(ended.kind) && outcome?.status === OperationStatus.SUCCEEDED) {
      this.alias.set('');
      this.language.set('');
      this.source.set('');
    }
  }
}
