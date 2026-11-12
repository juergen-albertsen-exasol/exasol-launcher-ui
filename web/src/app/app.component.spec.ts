import { DebugElement } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { provideNoopAnimations } from '@angular/platform-browser/animations';
import { Router, provideRouter, withComponentInputBinding } from '@angular/router';
import { provideStore } from '@ngxs/store';
import { NAV_ITEMS, NotificationsState } from '@exasol/ui-common';
import { AppComponent } from './app.component';
import { APP_NAV_ITEMS, appRoutes } from './routes';
import { DeploymentsState } from './store/deployments/deployments.state';
import { provideLocalApi, summary } from './testing/local-api.mock';

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
        provideRouter(appRoutes, withComponentInputBinding()),
        provideNoopAnimations(),
        provideStore([NotificationsState, DeploymentsState]),
        provideLocalApi({ deployments: [summary('default'), summary('test')] }),
        { provide: NAV_ITEMS, useValue: APP_NAV_ITEMS },
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

    it('shows the deployment selector in the header', () => {
      expect(root.query(By.css('header .controls exa-deployment-selector'))).toBeTruthy();
    });

    it('lists the Overview page of the deployment as the current navigation item', () => {
      const active = root.query(By.css('.nav-content-wrapper a.active'));

      expect(active.nativeElement.textContent).toContain('Overview');
      expect(active.nativeElement.getAttribute('href')).toBe('/deployments/test');
      expect(root.query(By.css('header h1')).nativeElement.textContent).toContain('Overview');
    });
  });
});
