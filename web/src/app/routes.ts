import { Route } from '@angular/router';
import { NavItem } from '@exasol/ui-common';
import { PageOverviewComponent } from './pages/page-overview/page-overview.component';

export const LocalRoutes = {
  overview: 'overview',
} as const;

const NavItemsKeys = {
  OVERVIEW: 'overview',
} as const;

export const APP_NAV_ITEMS: { [key: string]: NavItem } = {
  [NavItemsKeys.OVERVIEW]: {
    path: LocalRoutes.overview,
    faIcon: ['fa-database'],
    text: 'Overview',
  },
};

export const appRoutes: Route[] = [
  {
    path: LocalRoutes.overview,
    component: PageOverviewComponent,
    data: { title: APP_NAV_ITEMS[NavItemsKeys.OVERVIEW].text },
  },
  { path: '', redirectTo: LocalRoutes.overview, pathMatch: 'full' },
  { path: '**', redirectTo: LocalRoutes.overview },
];
