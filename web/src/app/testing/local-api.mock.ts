import { HttpErrorResponse } from '@angular/common/http';
import { of, throwError } from 'rxjs';
import {
  Configuration,
  DeploymentDetails,
  DeploymentSummary,
  Operation,
  OperationKind,
  PingResult,
  Sidecar,
} from '@exa/services/local-api/local-api.models';
import { LocalApiService } from '@exa/services/local-api/local-api.service';

export const summary = (name: string, status = 'database_ready'): DeploymentSummary => ({
  name,
  path: `/home/user/.exasol/personal/deployments/${name}`,
  status,
  infrastructure: 'local',
  installation: 'local',
});

export const runningDetails = (name: string, port = 8563): DeploymentDetails => ({
  name,
  state: 'running',
  connection: { host: '127.0.0.1', port, username: 'sys', insecure_skip_cert_validation: true },
});

export interface LocalApiFixture {
  deployments: DeploymentSummary[];
  details?: { [name: string]: DeploymentDetails };
  pings?: { [name: string]: PingResult };
  operations?: Operation[];
  configurations?: { [name: string]: Configuration };
  sidecars?: { [name: string]: Sidecar[] };
  /** Messages with which the API refuses to list a deployment's sidecars. */
  sidecarRefusals?: { [name: string]: string };
}

const conflict = (detail: string) =>
  throwError(() => new HttpErrorResponse({ status: 409, error: { title: 'Conflict', detail } }));

export const sidecar = (overrides: Partial<Sidecar> = {}): Sidecar => ({
  name: 'mcp',
  description: 'Exasol MCP server with read-oriented query tools',
  catalog: true,
  enabled: true,
  running: true,
  state: 'running',
  health: 'serving',
  restart_required: false,
  endpoints: [{ ip: '127.0.0.1', port: 4896, url: 'http://127.0.0.1:4896/mcp' }],
  ...overrides,
});

export const defaultSidecars = (): Sidecar[] => [
  sidecar(),
  sidecar({
    name: 'kafka',
    description: undefined,
    catalog: false,
    health: '',
    endpoints: [{ ip: '127.0.0.1', port: 19092 }],
  }),
  sidecar({
    name: 'dash-server',
    description: 'Dashboards built by AI agents',
    enabled: false,
    running: false,
    state: '',
    health: '',
    endpoints: [],
  }),
];

export const localConfiguration = (overrides: Partial<Configuration> = {}): Configuration => ({
  infrastructure: {
    name: 'local',
    display_name: 'Exasol Local',
    description: 'Exasol Local deployment using a macOS VM.',
    options: [
      { name: 'cpu-count', value: 2, secret: false },
      { name: 'data-size-gb', value: 100, secret: false },
      { name: 'memory-mb', value: 24576, secret: false },
      { name: 'ports', value: 'db:8563', secret: false },
    ],
  },
  installation: {
    name: 'local',
    display_name: 'Exasol Local',
    description: 'Uses the Exasol Local database container.',
    options: [],
  },
  editable: true,
  apply_with: 'start',
  ...overrides,
});

export const operation = (overrides: Partial<Operation> = {}): Operation => ({
  id: 'op-1',
  deployment: 'default',
  kind: 'start',
  status: 'running',
  started_at: new Date(Date.now() - 72_000).toISOString(),
  ...overrides,
});

export const mockLocalApi = (fixture: LocalApiFixture) => ({
  listDeployments: jest.fn(() => of(fixture.deployments)),
  getDeployment: jest.fn((name: string) => of(fixture.details?.[name] ?? runningDetails(name))),
  openFolder: jest.fn(() => of(undefined)),
  listOperations: jest.fn(() => of(fixture.operations ?? [])),
  getOperation: jest.fn((id: string) => of((fixture.operations ?? []).find(op => op.id === id) ?? operation({ id }))),
  runOperation: jest.fn((name: string, kind: OperationKind, sidecar?: string) =>
    of(operation({ id: 'new', deployment: name, kind, sidecar }))
  ),
  getConfiguration: jest.fn((name: string) => of(fixture.configurations?.[name] ?? localConfiguration())),
  setConfiguration: jest.fn((name: string) => of(fixture.configurations?.[name] ?? localConfiguration())),
  resetConfiguration: jest.fn((name: string) => of(fixture.configurations?.[name] ?? localConfiguration())),
  listSidecars: jest.fn((name: string) => {
    const refusal = fixture.sidecarRefusals?.[name];
    return refusal ? conflict(refusal) : of(fixture.sidecars?.[name] ?? defaultSidecars());
  }),
  pingDeployment: jest.fn((name: string) => of(fixture.pings?.[name] ?? { status: 'reachable', round_trip_ms: 5 })),
});

export const provideLocalApi = (fixture: LocalApiFixture) => ({
  provide: LocalApiService,
  useValue: mockLocalApi(fixture),
});
