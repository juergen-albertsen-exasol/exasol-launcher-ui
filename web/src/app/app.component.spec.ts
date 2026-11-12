import { DebugElement } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { provideNoopAnimations } from '@angular/platform-browser/animations';
import { Router, provideRouter } from '@angular/router';
import { provideStore } from '@ngxs/store';
import { NAV_ITEMS, NotificationsState } from '@exasol/ui-common';
import { AppComponent } from './app.component';
import { APP_NAV_ITEMS, appRoutes } from './routes';

describe('AppComponent', () => {
  let root: DebugElement;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [AppComponent],
      providers: [
        provideRouter(appRoutes),
        provideNoopAnimations(),
        provideStore([NotificationsState]),
        { provide: NAV_ITEMS, useValue: APP_NAV_ITEMS },
      ],
    }).compileComponents();

    const fixture = TestBed.createComponent(AppComponent);
    await TestBed.inject(Router).navigateByUrl('/');
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    root = fixture.debugElement;
  });

  it('shows the product name', () => {
    const logo = root.query(By.css('.logo'));

    expect(logo.nativeElement.textContent).toContain('Local');
  });

  it('opens the overview page as the current navigation item', () => {
    const active = root.query(By.css('.nav-content-wrapper a.active'));

    expect(active.nativeElement.textContent).toContain('Overview');
    expect(root.query(By.css('header h1')).nativeElement.textContent).toContain('Overview');
  });
});
