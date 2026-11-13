import { ComponentFixture, TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { provideRouter } from '@angular/router';
import { Store, provideStore } from '@ngxs/store';
import { firstValueFrom } from 'rxjs';
import { Operation } from '@exa/services/local-api/local-api.models';
import { LoadOperationsAction } from '@exa/store/operations/operations.actions';
import { OperationsState } from '@exa/store/operations/operations.state';
import { operation, provideLocalApi } from '../../testing/local-api.mock';
import { ActivityComponent } from './activity.component';

describe('ActivityComponent', () => {
  let fixture: ComponentFixture<ActivityComponent>;

  const render = async (operations: Operation[]) => {
    await TestBed.configureTestingModule({
      imports: [ActivityComponent],
      providers: [provideRouter([]), provideStore([OperationsState]), provideLocalApi({ deployments: [], operations })],
    }).compileComponents();
    await firstValueFrom(TestBed.inject(Store).dispatch(new LoadOperationsAction()));
    fixture = TestBed.createComponent(ActivityComponent);
    fixture.detectChanges();
  };

  const indicator = () => fixture.debugElement.query(By.css('.activity'));

  it('stays hidden while nothing runs', async () => {
    await render([operation({ status: 'succeeded' })]);

    expect(indicator()).toBeNull();
  });

  it('names the one running operation', async () => {
    await render([operation()]);

    expect(indicator().nativeElement.textContent.trim()).toBe('Starting default…');
  });

  it('counts several running operations', async () => {
    await render([operation({ id: 'a' }), operation({ id: 'b', deployment: 'test', kind: 'stop' })]);

    expect(indicator().nativeElement.textContent.trim()).toBe('2 operations running');
  });

  it('lists running and finished operations in its panel', async () => {
    await render([
      operation({ id: 'a' }),
      operation({
        id: 'b',
        deployment: 'test',
        kind: 'stop',
        status: 'succeeded',
        finished_at: new Date(Date.now() - 120_000).toISOString(),
      }),
    ]);

    indicator().nativeElement.click();
    fixture.detectChanges();

    const items = fixture.debugElement.queryAll(By.css('.activity__item'));
    expect(items).toHaveLength(2);
    expect(items[0].nativeElement.textContent).toContain('Starting default');
    expect(items[0].nativeElement.textContent).toContain('1m 12s');
    expect(items[0].query(By.css('a')).nativeElement.getAttribute('href')).toBe('/deployments/default/overview');
    expect(items[1].nativeElement.textContent).toContain('Stopped test');
    expect(items[1].nativeElement.textContent).toContain('2 min ago');
  });

  it('closes the panel on Escape', async () => {
    await render([operation()]);
    indicator().nativeElement.click();
    fixture.detectChanges();

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    fixture.detectChanges();

    expect(fixture.debugElement.query(By.css('.activity__panel'))).toBeNull();
  });
});
