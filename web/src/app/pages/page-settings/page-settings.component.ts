import { Component, computed, effect, inject, input, signal, untracked } from '@angular/core';
import { MatDialog } from '@angular/material/dialog';
import { RouterLink } from '@angular/router';
import { Store } from '@ngxs/store';
import { ConfirmationModalComponent, ExaButtonComponent } from '@exasol/ui-common';
import { DeploymentPage, deploymentPath } from '@exa/routes';
import {
  ConfigurationOption,
  ConfigurationPreset,
  OperationKind,
  OptionValue,
  PresetKind,
} from '@exa/services/local-api/local-api.models';
import { SelectDeploymentAction } from '@exa/store/deployments/deployments.actions';
import { RunOperationAction } from '@exa/store/operations/operations.actions';
import { LoadConfigurationAction, ResetOptionsAction, SaveOptionsAction } from '@exa/store/settings/settings.actions';
import { SettingsState } from '@exa/store/settings/settings.state';

type Drafts = { [preset in PresetKind]?: { [option: string]: OptionValue } };

const LOCAL_PRESET = 'local';

@Component({
  selector: 'exa-page-settings',
  templateUrl: './page-settings.component.html',
  styleUrls: ['./page-settings.component.scss'],
  imports: [ExaButtonComponent, RouterLink],
})
export class PageSettingsComponent {
  private readonly store = inject(Store);
  private readonly dialog = inject(MatDialog);

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

  private changedOptions(kind: PresetKind): { [option: string]: OptionValue } {
    const draft = this.drafts()[kind] ?? {};
    const options = this.preset(kind)?.options ?? [];
    return Object.fromEntries(
      Object.entries(draft).filter(([name, value]) => options.find(option => option.name === name)?.value !== value)
    );
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
