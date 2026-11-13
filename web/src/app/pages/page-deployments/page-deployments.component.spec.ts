import { DebugElement } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { Router, provideRouter } from '@angular/router';
import { Store, provideStore } from '@ngxs/store';
import { NEVER, firstValueFrom } from 'rxjs';
import { LocalApiService } from '@exa/services/local-api/local-api.service';
import { LoadDeploymentsAction } from '@exa/store/deployments/deployments.actions';
import { DeploymentsState } from '@exa/store/deployments/deployments.state';
import { OperationsState } from '@exa/store/operations/operations.state';
import { LocalApiFixture, mockLocalApi, provideLocalApi, summary } from '../../testing/local-api.mock';
import { PageDeploymentsComponent } from './page-deployments.component';

describe('PageDeploymentsComponent', () => {
  const render = async (fixture: LocalApiFixture): Promise<DebugElement> => {
    await TestBed.configureTestingModule({
      imports: [PageDeploymentsComponent],
      providers: [provideRouter([]), provideStore([DeploymentsState, OperationsState]), provideLocalApi(fixture)],
    }).compileComponents();

    await firstValueFrom(TestBed.inject(Store).dispatch(new LoadDeploymentsAction()));

    const component = TestBed.createComponent(PageDeploymentsComponent);
    component.detectChanges();
    return component.debugElement;
  };

  it('shows a tile with name, status, and infrastructure preset per deployment', async () => {
    const root = await render({ deployments: [summary('default'), summary('test', 'stopped')] });

    const tiles = root.queryAll(By.css('.deployments__tile'));
    const tileText = (tile: DebugElement, part: string) =>
      tile.query(By.css(`.deployments__${part}`)).nativeElement.textContent.trim();

    expect(tiles.map(tile => tileText(tile, 'name'))).toEqual(['default', 'test']);
    expect(tileText(tiles[1], 'status')).toBe('stopped');
    expect(tileText(tiles[1], 'preset')).toBe('local');
  });

  it('opens the overview of the selected tile', async () => {
    const root = await render({ deployments: [summary('default'), summary('test')] });
    const navigate = jest.spyOn(TestBed.inject(Router), 'navigateByUrl').mockResolvedValue(true);

    root.queryAll(By.css('.deployments__tile'))[1].nativeElement.click();

    expect(navigate.mock.calls[0][0].toString()).toBe('/deployments/test');
  });

  it('explains how to create a deployment when none is listed', async () => {
    const root = await render({ deployments: [] });

    expect(root.query(By.css('.deployments__message')).nativeElement.textContent).toContain('exasol install');
  });

  it('shows progress while the deployments are looked up', async () => {
    const api = mockLocalApi({ deployments: [] });
    api.listDeployments.mockReturnValue(NEVER);
    await TestBed.configureTestingModule({
      imports: [PageDeploymentsComponent],
      providers: [
        provideRouter([]),
        provideStore([DeploymentsState, OperationsState]),
        { provide: LocalApiService, useValue: api },
      ],
    }).compileComponents();
    TestBed.inject(Store).dispatch(new LoadDeploymentsAction());

    const component = TestBed.createComponent(PageDeploymentsComponent);
    component.detectChanges();

    const loading = component.debugElement.query(By.css('.deployments__loading'));
    expect(loading.query(By.css('mat-spinner'))).toBeTruthy();
    expect(loading.nativeElement.textContent).toContain('Looking up deployments');
  });

  it('links the Exasol logo to the start page', async () => {
    const root = await render({ deployments: [] });

    expect(root.query(By.css('.deployments__logo')).nativeElement.getAttribute('href')).toBe('/');
  });
});
