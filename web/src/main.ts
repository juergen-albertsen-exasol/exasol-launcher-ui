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
import { DiagnosticsState } from '@exa/store/diagnostics/diagnostics.state';
import { ImportsState } from '@exa/store/imports/imports.state';
import { OperationsState } from '@exa/store/operations/operations.state';
import { ScriptLanguagesState } from '@exa/store/script-languages/script-languages.state';
import { SettingsState } from '@exa/store/settings/settings.state';
import { SidecarsState } from '@exa/store/sidecars/sidecars.state';
import { SqlState } from '@exa/store/sql/sql.state';

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
    provideStore([
      NotificationsState,
      DeploymentsState,
      OperationsState,
      SettingsState,
      SidecarsState,
      ScriptLanguagesState,
      DiagnosticsState,
      SqlState,
      ImportsState,
    ]),
    { provide: NAV_ITEMS, useValue: APP_NAV_ITEMS },
  ],
}).catch(err => console.error(err));
