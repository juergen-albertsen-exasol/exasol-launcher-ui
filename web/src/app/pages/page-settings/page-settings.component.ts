import { NgTemplateOutlet } from '@angular/common';
import { Component, computed, effect, inject, input, signal, untracked } from '@angular/core';
import { MatDialog } from '@angular/material/dialog';
import { Router, RouterLink } from '@angular/router';
import { Store } from '@ngxs/store';
import { filter } from 'rxjs';
import { ConfirmationModalComponent, ExaButtonComponent } from '@exasol/ui-common';
import { DangerDialogComponent, DangerDialogData } from '@exa/components/danger-dialog/danger-dialog.component';
import { OperationProgressComponent } from '@exa/components/operation-progress/operation-progress.component';
import { DeploymentPage, LocalRoutes, deploymentPath } from '@exa/routes';
import {
  ConfigurationOption,
  ConfigurationPreset,
  Operation,
  OperationKind,
  OperationStatus,
  OptionValue,
  PresetKind,
} from '@exa/services/local-api/local-api.models';
import {
  DeploymentRemovedAction,
  LoadDeploymentsAction,
  SelectDeploymentAction,
} from '@exa/store/deployments/deployments.actions';
import { DeploymentsState } from '@exa/store/deployments/deployments.state';
import { runningLabel } from '@exa/store/operations/operation-labels';
import { RunOperationAction } from '@exa/store/operations/operations.actions';
import { OperationsState } from '@exa/store/operations/operations.state';
import {
  ChangesAppliedAction,
  LoadConfigurationAction,
  ResetOptionsAction,
  SaveOptionsAction,
} from '@exa/store/settings/settings.actions';
import { SettingsState } from '@exa/store/settings/settings.state';

type Drafts = { [preset in PresetKind]?: { [option: string]: OptionValue } };

const LOCAL_PRESET = 'local';
const NOT_INITIALIZED = 'not_initialized';
const REMOVING: OperationKind[] = [OperationKind.DESTROY_AND_REMOVE, OperationKind.REMOVE];

@Component({
  selector: 'exa-page-settings',
  templateUrl: './page-settings.component.html',
  styleUrls: ['./page-settings.component.scss'],
  imports: [ExaButtonComponent, RouterLink, NgTemplateOutlet, OperationProgressComponent],
})
export class PageSettingsComponent {
  private readonly store = inject(Store);
  private readonly dialog = inject(MatDialog);
  private readonly router = inject(Router);

  readonly name = input.required<string>();

  readonly presetKinds = Object.values(PresetKind);
  readonly boxLabel: Record<PresetKind, string> = {
    infrastructure: 'Infrastructure options',
    installation: 'Installation options',
  };
  readonly overviewPath = computed(() => deploymentPath(this.name(), DeploymentPage.OVERVIEW));

  private readonly configurations = this.store.selectSignal(SettingsState.configurations);
  private readonly loadErrors = this.store.selectSignal(SettingsState.loadErrors);
  private readonly savingPresets = this.store.selectSignal(SettingsState.saving);
  private readonly saveErrors = this.store.selectSignal(SettingsState.saveErrors);
  private readonly unappliedChanges = this.store.selectSignal(SettingsState.unapplied);

  readonly configuration = computed(() => this.configurations()[this.name()]);
  readonly loadError = computed(() => this.loadErrors()[this.name()]);
  readonly saving = computed(() => this.savingPresets()[this.name()]);
  readonly saveError = computed(() => this.saveErrors()[this.name()]);
  readonly unapplied = computed(() => !!this.unappliedChanges()[this.name()]);
  readonly local = computed(() => this.configuration()?.infrastructure.name === LOCAL_PRESET);
  readonly drafts = signal<Drafts>({});

  readonly runningLabel = runningLabel;
  readonly summary = this.store.selectSignal(DeploymentsState.selectedSummary);
  // Nothing exists to destroy before the deployment is initialized.
  readonly canDestroy = computed(() => this.summary()?.status !== NOT_INITIALIZED);
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

  constructor() {
    effect(() => {
      const name = this.name();
      untracked(() => {
        this.drafts.set({});
        this.store.dispatch([new SelectDeploymentAction(name), new LoadConfigurationAction(name)]);
      });
    });
    effect(() => {
      const configuration = this.configuration();
      if (configuration) {
        untracked(() => this.dropSavedDrafts());
      }
    });
    this.refreshWhenOperationEnds();
  }

  preset(kind: PresetKind): ConfigurationPreset | undefined {
    return this.configuration()?.[kind];
  }

  valueOf(kind: PresetKind, option: ConfigurationOption): OptionValue | undefined {
    return this.drafts()[kind]?.[option.name] ?? option.value;
  }

  typeOf(option: ConfigurationOption): 'secret' | 'boolean' | 'number' | 'text' {
    if (option.secret) {
      return 'secret';
    }
    switch (typeof option.value) {
      case 'boolean':
        return 'boolean';
      case 'number':
        return 'number';
      default:
        return 'text';
    }
  }

  edit(kind: PresetKind, option: ConfigurationOption, input: HTMLInputElement): void {
    let value: OptionValue = input.value;
    if (input.type === 'checkbox') {
      value = input.checked;
    } else if (input.type === 'number' && input.value !== '') {
      value = input.valueAsNumber;
    }
    this.drafts.update(drafts => ({ ...drafts, [kind]: { ...drafts[kind], [option.name]: value } }));
  }

  save(kind: PresetKind): void {
    const changed = this.changedOptions(kind);
    if (Object.keys(changed).length) {
      this.store.dispatch(new SaveOptionsAction(this.name(), kind, changed));
    }
  }

  resetToDefaults(kind: PresetKind): void {
    const name = this.name();
    const options = (this.preset(kind)?.options ?? []).filter(option => !option.secret).map(option => option.name);
    ConfirmationModalComponent.open(this.dialog, {
      title: `Reset the ${kind} options?`,
      content: `This resets ${options.join(', ')} of ${name} to the defaults of its ${kind} preset.`,
      focusOnCancel: true,
    })
      .afterClosed()
      .subscribe(confirmed => {
        if (confirmed) {
          this.drafts.update(drafts => ({ ...drafts, [kind]: {} }));
          this.store.dispatch(new ResetOptionsAction(name, kind, options));
        }
      });
  }

  run(kind: OperationKind): void {
    this.store.dispatch(new RunOperationAction(this.name(), kind));
  }

  confirmDanger(action: DangerDialogData['action'], removeFolder = false): void {
    const name = this.name();
    DangerDialogComponent.open(this.dialog, {
      deployment: name,
      path: this.summary()?.path ?? '',
      action,
      removeFolder,
    })
      .pipe(filter(kind => !!kind))
      .subscribe(kind => this.store.dispatch(new RunOperationAction(name, kind)));
  }

  private changedOptions(kind: PresetKind): { [option: string]: OptionValue } {
    const draft = this.drafts()[kind] ?? {};
    const options = this.preset(kind)?.options ?? [];
    return Object.fromEntries(
      Object.entries(draft).filter(([name, value]) => options.find(option => option.name === name)?.value !== value)
    );
  }

  private refreshWhenOperationEnds(): void {
    let previous: Operation | undefined;
    effect(() => {
      const running = this.running();
      const name = untracked(() => this.name());
      if (previous?.deployment === name && !running) {
        const ended = this.latest();
        untracked(() => this.operationEnded(name, ended));
      }
      previous = running;
    });
  }

  private operationEnded(name: string, ended: Operation | undefined): void {
    if (ended?.status !== OperationStatus.SUCCEEDED) {
      this.store.dispatch([new LoadDeploymentsAction(true), new LoadConfigurationAction(name)]);
      return;
    }
    this.store.dispatch(new ChangesAppliedAction(name));
    if (REMOVING.includes(ended.kind)) {
      this.store.dispatch(new DeploymentRemovedAction(name));
      this.router.navigate(['/', LocalRoutes.deployments]);
      return;
    }
    this.store.dispatch([new LoadDeploymentsAction(true), new LoadConfigurationAction(name)]);
  }

  private dropSavedDrafts(): void {
    const kept: Drafts = {};
    for (const kind of this.presetKinds) {
      const changed = this.changedOptions(kind);
      if (Object.keys(changed).length) {
        kept[kind] = changed;
      }
    }
    this.drafts.set(kept);
  }
}
