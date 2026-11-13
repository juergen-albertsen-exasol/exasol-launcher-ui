import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { LocalApiService } from './local-api.service';

describe('LocalApiService', () => {
  let service: LocalApiService;
  let http: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({ providers: [provideHttpClient(), provideHttpClientTesting()] });
    service = TestBed.inject(LocalApiService);
    http = TestBed.inject(HttpTestingController);
  });

  afterEach(() => http.verify());

  it('runs operations on their endpoints', () => {
    service.runOperation('default', 'deploy').subscribe();
    service.runOperation('default', 'destroy').subscribe();
    service.runOperation('default', 'destroy_and_remove').subscribe();
    service.runOperation('default', 'remove').subscribe();

    const requests = http.match(() => true);
    expect(requests.map(request => [request.request.method, request.request.url, request.request.body])).toEqual([
      ['POST', '/api/v1/deployments/default/deploy', null],
      ['POST', '/api/v1/deployments/default/destroy', null],
      ['POST', '/api/v1/deployments/default/destroy', { remove_folder: true }],
      ['POST', '/api/v1/deployments/default/remove', null],
    ]);
    requests.forEach(request => request.flush({}));
  });

  it('lists sidecars and runs sidecar operations on their endpoints', () => {
    service.listSidecars('default').subscribe();
    service.runOperation('default', 'enable_sidecar', { sidecar: 'dash-server' }).subscribe();
    service.runOperation('default', 'disable_sidecar', { sidecar: 'kafka' }).subscribe();
    service.runOperation('default', 'remove_sidecar', { sidecar: 'kafka' }).subscribe();

    const requests = http.match(() => true);
    expect(requests.map(request => [request.request.method, request.request.url, request.request.body])).toEqual([
      ['GET', '/api/v1/deployments/default/sidecars', null],
      ['POST', '/api/v1/deployments/default/sidecars/dash-server/enable', null],
      ['POST', '/api/v1/deployments/default/sidecars/kafka/disable', null],
      ['POST', '/api/v1/deployments/default/sidecars/kafka/disable', { remove_definition: true }],
    ]);
    requests.forEach(request => request.flush({}));
  });

  it('lists script languages and runs script language operations on their endpoints', () => {
    service.listScriptLanguages('default').subscribe();
    service.runOperation('default', 'install_script_language', { script_language: 'R' }).subscribe();
    service.runOperation('default', 'update_script_language', { script_language: 'RUST', restart: true }).subscribe();
    service.runOperation('default', 'remove_script_language', { script_language: 'JAVA', restart: false }).subscribe();
    const custom = { script_language: 'MYPY3', language: 'python', source: '/tmp/my.tar.gz', restart: false };
    service.runOperation('default', 'install_custom_script_language', custom).subscribe();
    service.runOperation('default', 'update_custom_script_language', custom).subscribe();

    const body = { alias: 'MYPY3', language: 'python', source: '/tmp/my.tar.gz', restart: false };
    const requests = http.match(() => true);
    expect(requests.map(request => [request.request.method, request.request.url, request.request.body])).toEqual([
      ['GET', '/api/v1/deployments/default/script-languages', null],
      ['POST', '/api/v1/deployments/default/script-languages/R/install', { restart: true }],
      ['POST', '/api/v1/deployments/default/script-languages/RUST/update', { restart: true }],
      ['POST', '/api/v1/deployments/default/script-languages/JAVA/remove', { restart: false }],
      ['POST', '/api/v1/deployments/default/script-languages/custom/install', body],
      ['POST', '/api/v1/deployments/default/script-languages/custom/update', body],
    ]);
    requests.forEach(request => request.flush({}));
  });

  it('reads the MCP client instructions', () => {
    service.listMcpClients('default').subscribe();
    service.getMcpInstructions('default', 'claude-code').subscribe();

    const requests = http.match(() => true);
    expect(requests.map(request => [request.request.method, request.request.url])).toEqual([
      ['GET', '/api/v1/deployments/default/mcp/clients'],
      ['GET', '/api/v1/deployments/default/mcp/clients/claude-code'],
    ]);
    requests.forEach(request => request.flush({}));
  });

  it('reads, patches, and resets the configuration', () => {
    service.getConfiguration('my deployment').subscribe();
    service.setConfiguration('default', { 'cpu-count': 4 }).subscribe();
    service.resetConfiguration('default', ['cpu-count']).subscribe();

    const requests = http.match(() => true);
    expect(requests.map(request => [request.request.method, request.request.url, request.request.body])).toEqual([
      ['GET', '/api/v1/deployments/my%20deployment/configuration', null],
      ['PATCH', '/api/v1/deployments/default/configuration', { options: { 'cpu-count': 4 } }],
      ['POST', '/api/v1/deployments/default/configuration/reset', { options: ['cpu-count'] }],
    ]);
    requests.forEach(request => request.flush({}));
  });

  it('reads diagnostics from their endpoints', () => {
    service.getLocalRuntime('my deployment').subscribe();
    service.getDiagnosticReport('default').subscribe();

    const requests = http.match(() => true);
    expect(requests.map(request => [request.request.method, request.request.url])).toEqual([
      ['GET', '/api/v1/deployments/my%20deployment/diagnostics/local'],
      ['GET', '/api/v1/deployments/default/diagnostics/info'],
    ]);
    requests.forEach(request => request.flush({}));
  });
});
