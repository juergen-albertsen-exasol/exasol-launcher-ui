import { provideHttpClient, withInterceptors } from '@angular/common/http';
import { importProvidersFrom, provideZoneChangeDetection } from '@angular/core';
import { MatSnackBarModule } from '@angular/material/snack-bar';
import { bootstrapApplication } from '@angular/platform-browser';
import { provideAnimations } from '@angular/platform-browser/animations';
import { provideRouter, withComponentInputBinding, withEnabledBlockingInitialNavigation } from '@angular/router';
import { provideStore } from '@ngxs/store';
import { NAV_ITEMS, NotificationsState } from '@exasol/ui-common';
import { AppComponent } from '@exa/app.component';
import { APP_NAV_ITEMS, appRoutes, withAppRouterConfig } from '@exa/routes';
import { sessionInterceptor } from '@exa/services/session/session.interceptor';
import { DeploymentsState } from '@exa/store/deployments/deployments.state';
import { OperationsState } from '@exa/store/operations/operations.state';
import { SettingsState } from '@exa/store/settings/settings.state';

bootstrapApplication(AppComponent, {
  providers: [
    provideZoneChangeDetection(),
    importProvidersFrom(MatSnackBarModule),
    provideRouter(
      appRoutes,
      withEnabledBlockingInitialNavigation(),
      withComponentInputBinding(),
      withAppRouterConfig()
    ),
    provideHttpClient(withInterceptors([sessionInterceptor])),
    provideAnimations(),
    provideStore([NotificationsState, DeploymentsState, OperationsState, SettingsState]),
    { provide: NAV_ITEMS, useValue: APP_NAV_ITEMS },
  ],
}).catch(err => console.error(err));
