import { Component, computed, effect, inject, input, untracked } from '@angular/core';
import { Store } from '@ngxs/store';
import { SelectDeploymentAction } from '@exa/store/deployments/deployments.actions';
import { LoadImportsAction } from '@exa/store/imports/imports.actions';
import { ImportsState, emptyImportPage } from '@exa/store/imports/imports.state';

@Component({
  selector: 'exa-page-import',
  templateUrl: './page-import.component.html',
  styleUrls: ['./page-import.component.scss'],
})
export class PageImportComponent {
  private readonly store = inject(Store);
  private readonly pages = this.store.selectSignal(ImportsState.pages);

  readonly name = input.required<string>();
  readonly page = computed(() => this.pages()[this.name()] ?? emptyImportPage());

  constructor() {
    effect(() => {
      const name = this.name();
      untracked(() => {
        this.store.dispatch(new SelectDeploymentAction(name));
        this.store.dispatch(new LoadImportsAction(name));
      });
    });
  }
}
