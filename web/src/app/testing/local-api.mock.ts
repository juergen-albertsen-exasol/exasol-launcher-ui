import { HttpErrorResponse } from '@angular/common/http';
import { of, throwError } from 'rxjs';
import {
  Configuration,
  DeploymentDetails,
  DeploymentSummary,
  DiagnosticReport,
  LocalRuntime,
  McpClient,
  McpInstructions,
  Operation,
  OperationKind,
  OperationParameters,
  PingResult,
  ScriptLanguages,
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
  scriptLanguages?: { [name: string]: ScriptLanguages };
  /** Messages with which the API refuses to list a deployment's script languages. */
  scriptLanguageRefusals?: { [name: string]: string };
  localRuntimes?: { [name: string]: LocalRuntime };
  /** Messages with which the API refuses to report a deployment's local runtime. */
  localRuntimeRefusals?: { [name: string]: string };
  reports?: { [name: string]: DiagnosticReport };
}

export const MCP_CLIENTS: McpClient[] = [
  { id: 'claude-code', name: 'Claude Code' },
  { id: 'claude-desktop', name: 'Claude Desktop' },
  { id: 'cursor', name: 'Cursor' },
];

export const mcpInstructions = (client: string): McpInstructions =>
  client === 'cursor'
    ? {
        id: 'cursor',
        name: 'Cursor',
        url: 'http://127.0.0.1:4896/mcp',
        file: '~/.cursor/mcp.json',
        content: '{\n  "mcpServers": {\n    "exasol-personal": { "url": "http://127.0.0.1:4896/mcp" }\n  }\n}\n',
      }
    : {
        id: client,
        name: MCP_CLIENTS.find(entry => entry.id === client)?.name ?? client,
        url: 'http://127.0.0.1:4896/mcp',
        command: 'claude mcp add --transport http --scope user exasol-personal http://127.0.0.1:4896/mcp',
      };

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

export const defaultScriptLanguages = (): ScriptLanguages => ({
  official: [
    {
      alias: 'JAVA',
      language: 'java',
      flavor: 'java-17',
      version: '11.2.0',
      aliases: ['JAVA', 'JAVA17'],
      installed: true,
      latest_release: false,
    },
    {
      alias: 'PYTHON3',
      language: 'python',
      flavor: 'python-3.12',
      version: '11.2.0',
      aliases: ['PYTHON3', 'PYTHON312'],
      installed: true,
      latest_release: false,
    },
    {
      alias: 'R',
      language: 'r',
      flavor: 'r-4.4',
      version: '11.2.0',
      aliases: ['R', 'R44'],
      installed: false,
      latest_release: false,
    },
    { alias: 'RUST', language: 'rust', flavor: '', aliases: ['RUST'], installed: false, latest_release: true },
  ],
  custom: [{ alias: 'MYPY3', language: 'python', source: '/Users/me/containers/my-python.tar.gz', available: true }],
});

export const runningLocalRuntime = (): LocalRuntime => ({
  platform: 'darwin/arm64',
  platform_supported: true,
  vm_running: true,
  database_ready: true,
  ports: [
    { name: 'db', host_port: 8563, reachability: 'reachable' },
    { name: 'sidecar mcp', host_port: 4896, reachability: 'reachable' },
  ],
});

export const stoppedLocalRuntime = (): LocalRuntime => ({
  platform: 'darwin/arm64',
  platform_supported: true,
  vm_running: false,
  message:
    'The platform is ready to run the local deployment. Run `exasol start` to start it, ' +
    'then run `exasol diag local` again for more detail.',
  ports: [],
});

export const diagnosticReport = (): DiagnosticReport => ({
  backend: 'local',
  deploymentState: 'running',
  connection: { host: '127.0.0.1', dbPort: 8563, username: 'sys' },
});

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
  runOperation: jest.fn((name: string, kind: OperationKind, params?: OperationParameters) =>
    of(
      operation({
        id: 'new',
        deployment: name,
        kind,
        sidecar: params?.sidecar,
        script_language: params?.script_language,
      })
    )
  ),
  getConfiguration: jest.fn((name: string) => of(fixture.configurations?.[name] ?? localConfiguration())),
  setConfiguration: jest.fn((name: string) => of(fixture.configurations?.[name] ?? localConfiguration())),
  resetConfiguration: jest.fn((name: string) => of(fixture.configurations?.[name] ?? localConfiguration())),
  listSidecars: jest.fn((name: string) => {
    const refusal = fixture.sidecarRefusals?.[name];
    return refusal ? conflict(refusal) : of(fixture.sidecars?.[name] ?? defaultSidecars());
  }),
  listScriptLanguages: jest.fn((name: string) => {
    const refusal = fixture.scriptLanguageRefusals?.[name];
    return refusal ? conflict(refusal) : of(fixture.scriptLanguages?.[name] ?? defaultScriptLanguages());
  }),
  getLocalRuntime: jest.fn((name: string) => {
    const refusal = fixture.localRuntimeRefusals?.[name];
    return refusal ? conflict(refusal) : of(fixture.localRuntimes?.[name] ?? runningLocalRuntime());
  }),
  getDiagnosticReport: jest.fn((name: string) => of(fixture.reports?.[name] ?? diagnosticReport())),
  listMcpClients: jest.fn(() => of(MCP_CLIENTS)),
  getMcpInstructions: jest.fn((name: string, client: string) => of(mcpInstructions(client))),
  pingDeployment: jest.fn((name: string) => of(fixture.pings?.[name] ?? { status: 'reachable', round_trip_ms: 5 })),
});

export const provideLocalApi = (fixture: LocalApiFixture) => ({
  provide: LocalApiService,
  useValue: mockLocalApi(fixture),
});
