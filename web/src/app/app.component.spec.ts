import { DebugElement } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { provideNoopAnimations } from '@angular/platform-browser/animations';
import { Router, provideRouter, withComponentInputBinding } from '@angular/router';
import { Store, provideStore } from '@ngxs/store';
import { firstValueFrom } from 'rxjs';
import { NAV_ITEMS, NotificationsState } from '@exasol/ui-common';
import { OperationsPollingService } from '@exa/store/operations/operations-polling.service';
import { OperationsState } from '@exa/store/operations/operations.state';
import { AppComponent } from './app.component';
import { APP_NAV_ITEMS, appRoutes, withAppRouterConfig } from './routes';
import { SessionService } from './services/session/session.service';
import { DeploymentsState } from './store/deployments/deployments.state';
import { LoadOperationsAction } from './store/operations/operations.actions';
import { SettingsState } from './store/settings/settings.state';
import { SidecarsState } from './store/sidecars/sidecars.state';
import { operation, provideLocalApi, summary } from './testing/local-api.mock';

describe('AppComponent', () => {
  let fixture: ComponentFixture<AppComponent>;
  let root: DebugElement;

  const open = async (url: string) => {
    await TestBed.inject(Router).navigateByUrl(url);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
  };

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [AppComponent],
      providers: [
        provideRouter(appRoutes, withComponentInputBinding(), withAppRouterConfig()),
        provideNoopAnimations(),
        provideStore([NotificationsState, DeploymentsState, OperationsState, SettingsState, SidecarsState]),
        provideLocalApi({ deployments: [summary('default'), summary('test')], operations: [operation()] }),
        { provide: NAV_ITEMS, useValue: APP_NAV_ITEMS },
        { provide: OperationsPollingService, useValue: { start: jest.fn() } },
      ],
    }).compileComponents();

    fixture = TestBed.createComponent(AppComponent);
    root = fixture.debugElement;
  });

  it('opens on the deployment tiles without navigation', async () => {
    await open('/');

    expect(root.queryAll(By.css('.deployments__tile'))).toHaveLength(2);
    expect(root.query(By.css('exa-layout'))).toBeNull();
  });

  describe('on an overview page', () => {
    beforeEach(() => open('/deployments/test'));

    it('shows the Exasol logo without a product suffix', () => {
      const logo = root.query(By.css('.logo'));

      expect(logo.query(By.css('.logo_full_light'))).toBeTruthy();
      expect(logo.nativeElement.textContent.trim()).toBe('');
    });

    it('returns to the Deployments page from the Exasol logo', async () => {
      root.query(By.css('.logo')).nativeElement.click();
      await fixture.whenStable();
      fixture.detectChanges();

      expect(TestBed.inject(Router).url).toBe('/deployments');
      expect(root.query(By.css('exa-page-deployments'))).toBeTruthy();
    });

    it('shows a running operation in the header', async () => {
      await firstValueFrom(TestBed.inject(Store).dispatch(new LoadOperationsAction()));
      fixture.detectChanges();

      const indicator = root.query(By.css('header .controls exa-activity .activity'));
      expect(indicator.nativeElement.textContent).toContain('Starting default');
    });

    it('shows the deployment selector in the header', () => {
      expect(root.query(By.css('header .controls exa-deployment-selector'))).toBeTruthy();
    });

    it('lists the pages of the deployment, with Overview as the current page', () => {
      const links = root.queryAll(By.css('.nav-content-wrapper a[href^="/deployments"]'));
      const active = root.queryAll(By.css('.nav-content-wrapper a.active'));

      expect(links.map(link => link.nativeElement.textContent.trim())).toEqual(['Overview', 'Sidecars', 'Settings']);
      expect(active.map(link => link.nativeElement.getAttribute('href'))).toEqual(['/deployments/test/overview']);
      expect(root.query(By.css('header h1')).nativeElement.textContent).toContain('Overview');
    });

    it('opens the Sidecars page from the navigation drawer', async () => {
      root.query(By.css('.nav-content-wrapper a[href="/deployments/test/sidecars"]')).nativeElement.click();
      await fixture.whenStable();
      fixture.detectChanges();

      expect(TestBed.inject(Router).url).toBe('/deployments/test/sidecars');
      expect(root.query(By.css('exa-page-sidecars'))).toBeTruthy();
      expect(root.query(By.css('header h1')).nativeElement.textContent).toContain('Sidecars');
    });

    it('opens the Settings page from the navigation drawer', async () => {
      const settings = root.query(By.css('.nav-content-wrapper a[href="/deployments/test/settings"]'));
      settings.nativeElement.click();
      await fixture.whenStable();
      fixture.detectChanges();

      const active = root.queryAll(By.css('.nav-content-wrapper a.active'));
      expect(TestBed.inject(Router).url).toBe('/deployments/test/settings');
      expect(root.query(By.css('exa-page-settings'))).toBeTruthy();
      expect(active.map(link => link.nativeElement.textContent.trim())).toEqual(['Settings']);
      expect(root.query(By.css('header h1')).nativeElement.textContent).toContain('Settings');
    });
  });

  it('tells the user to reconnect when the session expires', async () => {
    await open('/');
    TestBed.inject(SessionService).expire();
    fixture.detectChanges();

    const message = root.query(By.css('exa-session-expired'));
    expect(message.nativeElement.textContent).toContain('Open the access URL');
    expect(root.query(By.css('exa-page-deployments'))).toBeNull();
  });
});
