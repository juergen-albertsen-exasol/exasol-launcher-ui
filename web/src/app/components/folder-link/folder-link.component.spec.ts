import { HttpErrorResponse } from '@angular/common/http';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { Store, provideStore } from '@ngxs/store';
import { throwError } from 'rxjs';
import { ErrorNotificationsAction } from '@exasol/ui-common';
import { LocalApiService } from '@exa/services/local-api/local-api.service';
import { mockLocalApi } from '../../testing/local-api.mock';
import { FolderLinkComponent, fileUrl } from './folder-link.component';

describe('fileUrl', () => {
  it('converts POSIX paths', () => {
    expect(fileUrl('/Users/me/.exasol/personal/deployments/my test')).toBe(
      'file:///Users/me/.exasol/personal/deployments/my%20test'
    );
  });

  it('converts Windows paths', () => {
    expect(fileUrl('C:\\Users\\me\\.exasol\\personal\\deployments\\default')).toBe(
      'file:///C:/Users/me/.exasol/personal/deployments/default'
    );
  });
});

describe('FolderLinkComponent', () => {
  let fixture: ComponentFixture<FolderLinkComponent>;
  let api: ReturnType<typeof mockLocalApi>;

  beforeEach(async () => {
    api = mockLocalApi({ deployments: [] });
    await TestBed.configureTestingModule({
      imports: [FolderLinkComponent],
      providers: [provideStore([]), { provide: LocalApiService, useValue: api }],
    }).compileComponents();

    fixture = TestBed.createComponent(FolderLinkComponent);
    fixture.componentRef.setInput('deployment', 'default');
    fixture.componentRef.setInput('path', '/deployments/default');
    fixture.detectChanges();
  });

  const link = () => fixture.debugElement.query(By.css('a')).nativeElement as HTMLAnchorElement;

  it('shows the path with its file URL as the address', () => {
    expect(link().textContent?.trim()).toBe('/deployments/default');
    expect(link().getAttribute('href')).toBe('file:///deployments/default');
  });

  it('asks the server to open the folder instead of navigating', () => {
    const click = new MouseEvent('click', { cancelable: true });

    link().dispatchEvent(click);

    expect(click.defaultPrevented).toBe(true);
    expect(api.openFolder).toHaveBeenCalledWith('default');
  });

  it('reports a failure as a notification', () => {
    const dispatch = jest.spyOn(TestBed.inject(Store), 'dispatch');
    api.openFolder.mockReturnValue(
      throwError(() => new HttpErrorResponse({ status: 500, error: { detail: 'no file manager' } }))
    );

    link().click();

    expect(dispatch).toHaveBeenCalledWith(new ErrorNotificationsAction('Cannot open the folder: no file manager'));
  });
});
