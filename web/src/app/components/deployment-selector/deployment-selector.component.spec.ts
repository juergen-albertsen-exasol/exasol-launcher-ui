import { OverlayContainer } from '@angular/cdk/overlay';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { provideNoopAnimations } from '@angular/platform-browser/animations';
import { Router, provideRouter } from '@angular/router';
import { Store, provideStore } from '@ngxs/store';
import { firstValueFrom } from 'rxjs';
import { LoadDeploymentsAction, SelectDeploymentAction } from '@exa/store/deployments/deployments.actions';
import { DeploymentsState } from '@exa/store/deployments/deployments.state';
import { provideLocalApi, summary } from '../../testing/local-api.mock';
import { DeploymentSelectorComponent } from './deployment-selector.component';

describe('DeploymentSelectorComponent', () => {
  let fixture: ComponentFixture<DeploymentSelectorComponent>;
  let overlayContainerElement: HTMLElement;
  let store: Store;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [DeploymentSelectorComponent],
      providers: [
        provideNoopAnimations(),
        provideRouter([]),
        provideStore([DeploymentsState]),
        provideLocalApi({ deployments: [summary('default'), summary('test')] }),
      ],
    }).compileComponents();

    store = TestBed.inject(Store);
    await firstValueFrom(store.dispatch(new LoadDeploymentsAction()));
    await firstValueFrom(store.dispatch(new SelectDeploymentAction('default')));

    overlayContainerElement = TestBed.inject(OverlayContainer).getContainerElement();
    fixture = TestBed.createComponent(DeploymentSelectorComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
  });

  const trigger = () => fixture.debugElement.query(By.css('.mat-mdc-select-trigger'));

  it('shows the selected deployment', () => {
    expect(trigger().nativeElement.textContent).toContain('default');
  });

  it('lists every deployment and opens the overview of the chosen one', async () => {
    const navigate = jest.spyOn(TestBed.inject(Router), 'navigate').mockResolvedValue(true);
    trigger().nativeElement.click();
    fixture.detectChanges();

    const options = Array.from(overlayContainerElement.querySelectorAll('mat-option')) as HTMLElement[];
    expect(options.map(option => option.textContent?.trim())).toEqual(['default', 'test']);

    options[1].click();
    fixture.detectChanges();
    await fixture.whenStable();

    expect(navigate).toHaveBeenCalledWith(['/', 'deployments', 'test']);
  });
});
