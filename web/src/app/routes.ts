import { ActivatedRouteSnapshot, Route, withRouterConfig } from '@angular/router';
import { NavItem } from '@exasol/ui-common';
import { PageDeploymentsComponent } from './pages/page-deployments/page-deployments.component';
import { PageMcpComponent } from './pages/page-mcp/page-mcp.component';
import { PageOverviewComponent } from './pages/page-overview/page-overview.component';
import { PageScriptLanguagesComponent } from './pages/page-script-languages/page-script-languages.component';
import { PageSettingsComponent } from './pages/page-settings/page-settings.component';
import { PageSidecarsComponent } from './pages/page-sidecars/page-sidecars.component';
import { ShellComponent } from './shell/shell.component';

export const LocalRoutes = {
  deployments: 'deployments',
} as const;

export const DeploymentPage = {
  OVERVIEW: 'overview',
  SIDECARS: 'sidecars',
  MCP: 'mcp',
  SCRIPT_LANGUAGES: 'script-languages',
  SETTINGS: 'settings',
} as const;
export type DeploymentPage = (typeof DeploymentPage)[keyof typeof DeploymentPage];

export const deploymentPath = (name: string, page: DeploymentPage = DeploymentPage.OVERVIEW): string[] => [
  '/',
  LocalRoutes.deployments,
  name,
  page,
];

export const overviewPath = (name: string): string[] => deploymentPath(name);

// The layout reads these paths on every render, so updating them retargets the
// navigation at the selected deployment. Its links match by prefix, so every
// page has its own path segment.
export const APP_NAV_ITEMS: { [key: string]: NavItem } = {
  [DeploymentPage.OVERVIEW]: {
    path: '',
    faIcon: ['fa-database'],
    text: 'Overview',
  },
  [DeploymentPage.SIDECARS]: {
    path: '',
    faIcon: ['fa-cubes'],
    text: 'Sidecars',
  },
  [DeploymentPage.MCP]: {
    path: '',
    faIcon: ['fa-robot'],
    text: 'MCP server',
  },
  [DeploymentPage.SCRIPT_LANGUAGES]: {
    path: '',
    faIcon: ['fa-code'],
    text: 'Script languages',
  },
  [DeploymentPage.SETTINGS]: {
    path: '',
    faIcon: ['fa-sliders'],
    text: 'Settings',
  },
};

export const setNavDeployment = (name: string): void => {
  for (const page of Object.values(DeploymentPage)) {
    APP_NAV_ITEMS[page].path = `/${LocalRoutes.deployments}/${encodeURIComponent(name)}/${page}`;
  }
};

/** Pages read the deployment name of the shell route. */
export const withAppRouterConfig = () => withRouterConfig({ paramsInheritanceStrategy: 'always' });

// The layout shows the title of the top route, so it takes the current page's.
const pageTitle = (route: ActivatedRouteSnapshot): string => route.firstChild?.data['title'] ?? '';

const page = (path: DeploymentPage, component: Route['component']): Route => ({
  path,
  component,
  data: { title: APP_NAV_ITEMS[path].text },
});

export const appRoutes: Route[] = [
  {
    path: LocalRoutes.deployments,
    component: PageDeploymentsComponent,
  },
  {
    path: `${LocalRoutes.deployments}/:name`,
    component: ShellComponent,
    resolve: { title: pageTitle },
    runGuardsAndResolvers: 'always',
    children: [
      page(DeploymentPage.OVERVIEW, PageOverviewComponent),
      page(DeploymentPage.SIDECARS, PageSidecarsComponent),
      page(DeploymentPage.MCP, PageMcpComponent),
      page(DeploymentPage.SCRIPT_LANGUAGES, PageScriptLanguagesComponent),
      page(DeploymentPage.SETTINGS, PageSettingsComponent),
      { path: '', redirectTo: DeploymentPage.OVERVIEW, pathMatch: 'full' },
    ],
  },
  { path: '', redirectTo: LocalRoutes.deployments, pathMatch: 'full' },
  { path: '**', redirectTo: LocalRoutes.deployments },
];
