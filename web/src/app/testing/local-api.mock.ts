import { of } from 'rxjs';
import {
  DeploymentDetails,
  DeploymentSummary,
  Operation,
  OperationKind,
  PingResult,
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
}

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
  runOperation: jest.fn((name: string, kind: OperationKind) => of(operation({ id: 'new', deployment: name, kind }))),
  pingDeployment: jest.fn((name: string) => of(fixture.pings?.[name] ?? { status: 'reachable', round_trip_ms: 5 })),
});

export const provideLocalApi = (fixture: LocalApiFixture) => ({
  provide: LocalApiService,
  useValue: mockLocalApi(fixture),
});
