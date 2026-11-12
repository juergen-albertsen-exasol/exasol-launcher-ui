import { HttpClient, provideHttpClient, withInterceptors } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { sessionInterceptor } from './session.interceptor';
import { SessionService } from './session.service';

describe('sessionInterceptor', () => {
  let http: HttpClient;
  let backend: HttpTestingController;
  let session: SessionService;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [provideHttpClient(withInterceptors([sessionInterceptor])), provideHttpClientTesting()],
    });
    http = TestBed.inject(HttpClient);
    backend = TestBed.inject(HttpTestingController);
    session = TestBed.inject(SessionService);
  });

  it('expires the session on status 401 and passes the error on', () => {
    const errors: unknown[] = [];
    http.get('/api/v1/deployments').subscribe({ error: error => errors.push(error) });

    backend.expectOne('/api/v1/deployments').flush(null, { status: 401, statusText: 'Unauthorized' });

    expect(session.expired()).toBe(true);
    expect(errors).toHaveLength(1);
  });

  it('keeps the session on other errors', () => {
    http.get('/api/v1/deployments').subscribe({ error: () => undefined });

    backend.expectOne('/api/v1/deployments').flush(null, { status: 500, statusText: 'Internal Server Error' });

    expect(session.expired()).toBe(false);
  });
});
