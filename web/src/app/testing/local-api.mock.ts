import { of } from 'rxjs';
import { DeploymentDetails, DeploymentSummary, PingResult } from '@exa/services/local-api/local-api.models';
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
}

export const mockLocalApi = (fixture: LocalApiFixture) => ({
  listDeployments: jest.fn(() => of(fixture.deployments)),
  getDeployment: jest.fn((name: string) => of(fixture.details?.[name] ?? runningDetails(name))),
  pingDeployment: jest.fn((name: string) => of(fixture.pings?.[name] ?? { status: 'reachable', round_trip_ms: 5 })),
});

export const provideLocalApi = (fixture: LocalApiFixture) => ({
  provide: LocalApiService,
  useValue: mockLocalApi(fixture),
});
