import { Route } from '@angular/router';
import { NavItem } from '@exasol/ui-common';
import { PageDeploymentsComponent } from './pages/page-deployments/page-deployments.component';
import { PageOverviewComponent } from './pages/page-overview/page-overview.component';
import { ShellComponent } from './shell/shell.component';

export const LocalRoutes = {
  deployments: 'deployments',
} as const;

const NavItemsKeys = {
  OVERVIEW: 'overview',
} as const;

export const overviewPath = (name: string): string[] => ['/', LocalRoutes.deployments, name];

// The layout reads these paths on every render, so updating them retargets the
// navigation at the selected deployment.
export const APP_NAV_ITEMS: { [key: string]: NavItem } = {
  [NavItemsKeys.OVERVIEW]: {
    path: '',
    faIcon: ['fa-database'],
    text: 'Overview',
  },
};

export const setNavDeployment = (name: string): void => {
  APP_NAV_ITEMS[NavItemsKeys.OVERVIEW].path = `/${LocalRoutes.deployments}/${encodeURIComponent(name)}`;
};

export const appRoutes: Route[] = [
  {
    path: LocalRoutes.deployments,
    component: PageDeploymentsComponent,
  },
  {
    path: `${LocalRoutes.deployments}/:name`,
    component: ShellComponent,
    data: { title: APP_NAV_ITEMS[NavItemsKeys.OVERVIEW].text },
    children: [{ path: '', component: PageOverviewComponent }],
  },
  { path: '', redirectTo: LocalRoutes.deployments, pathMatch: 'full' },
  { path: '**', redirectTo: LocalRoutes.deployments },
];
