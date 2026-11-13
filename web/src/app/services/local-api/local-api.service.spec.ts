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
});
