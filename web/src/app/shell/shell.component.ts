import { Component, ElementRef, afterNextRender, effect, inject, input, viewChild } from '@angular/core';
import { RouterModule } from '@angular/router';
import { LayoutComponent } from '@exasol/ui-common';
import { ActivityComponent } from '@exa/components/activity/activity.component';
import { DeploymentSelectorComponent } from '@exa/components/deployment-selector/deployment-selector.component';
import { setNavDeployment } from '@exa/routes';

@Component({
  selector: 'exa-shell',
  templateUrl: './shell.component.html',
  styleUrls: ['./shell.component.scss'],
  imports: [LayoutComponent, RouterModule, DeploymentSelectorComponent, ActivityComponent],
})
export class ShellComponent {
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly selector = viewChild.required(DeploymentSelectorComponent, { read: ElementRef });
  private readonly activity = viewChild.required(ActivityComponent, { read: ElementRef });

  readonly name = input.required<string>();

  constructor() {
    effect(() => setNavDeployment(this.name()));
    // The layout offers no header slot, so the activity indicator and the
    // selector join the header's controls once the layout has rendered.
    afterNextRender(() => {
      this.host.nativeElement
        .querySelector('header .controls')
        ?.prepend(this.activity().nativeElement, this.selector().nativeElement);
    });
  }
}
