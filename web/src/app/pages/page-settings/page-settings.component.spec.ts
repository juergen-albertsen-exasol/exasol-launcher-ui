import { HttpErrorResponse } from '@angular/common/http';
import { DebugElement } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { MatDialog } from '@angular/material/dialog';
import { By } from '@angular/platform-browser';
import { provideNoopAnimations } from '@angular/platform-browser/animations';
import { provideRouter } from '@angular/router';
import { Store, provideStore } from '@ngxs/store';
import { firstValueFrom, of, throwError } from 'rxjs';
import { Configuration } from '@exa/services/local-api/local-api.models';
import { LocalApiService } from '@exa/services/local-api/local-api.service';
import { LoadDeploymentsAction } from '@exa/store/deployments/deployments.actions';
import { DeploymentsState } from '@exa/store/deployments/deployments.state';
import { OperationsState } from '@exa/store/operations/operations.state';
import { SettingsState } from '@exa/store/settings/settings.state';
import { LocalApiFixture, localConfiguration, mockLocalApi, summary } from '../../testing/local-api.mock';
import { PageSettingsComponent } from './page-settings.component';

const awsConfiguration = (): Configuration => ({
  infrastructure: {
    name: 'aws',
    display_name: 'Exasol in AWS',
    description: 'Provisioning in AWS.',
    options: [
      { name: 'cluster-size', value: 1, secret: false },
      { name: 'db-password', secret: true },
      { name: 's3-archive-enabled', value: true, secret: false },
    ],
  },
  installation: {
    name: 'ubuntu',
    display_name: 'Exasol on Ubuntu',
    description: 'Installs Exasol on Ubuntu.',
    options: [{ name: 'exasol-version', value: '2026.1.0', secret: false }],
  },
  editable: true,
  apply_with: 'deploy',
});

const withOption = (name: string, value: number): Configuration => {
  const configuration = localConfiguration();
  configuration.infrastructure.options = configuration.infrastructure.options.map(option =>
    option.name === name ? { ...option, value } : option
  );
  return configuration;
};

describe('PageSettingsComponent', () => {
  let component: ComponentFixture<PageSettingsComponent>;
  let api: ReturnType<typeof mockLocalApi>;
  let confirmed: boolean;

  const refresh = async () => {
    component.detectChanges();
    await component.whenStable();
    component.detectChanges();
  };

  const render = async (fixture: LocalApiFixture, name = 'default'): Promise<DebugElement> => {
    api = mockLocalApi(fixture);
    confirmed = true;
    await TestBed.configureTestingModule({
      imports: [PageSettingsComponent],
      providers: [
        provideNoopAnimations(),
        provideRouter([]),
        provideStore([DeploymentsState, OperationsState, SettingsState]),
        { provide: LocalApiService, useValue: api },
        { provide: MatDialog, useValue: { open: jest.fn(() => ({ afterClosed: () => of(confirmed) })) } },
      ],
    }).compileComponents();

    await firstValueFrom(TestBed.inject(Store).dispatch(new LoadDeploymentsAction()));
    component = TestBed.createComponent(PageSettingsComponent);
    component.componentRef.setInput('name', name);
    await refresh();
    return component.debugElement;
  };

  const box = (root: DebugElement, label: string) => root.query(By.css(`.settings__box[data-label="${label}"]`));
  const field = (root: DebugElement, id: string) => root.query(By.css(`#${id}`)).nativeElement as HTMLInputElement;
  const type = (input: HTMLInputElement, value: string) => {
    input.value = value;
    input.dispatchEvent(new Event('input'));
  };
  const click = (root: DebugElement, selector: string) => {
    const element = root.query(By.css(selector)).nativeElement as HTMLElement;
    (element.querySelector('button') ?? element).click();
  };

  it('shows the presets with their descriptions and the options with their values', async () => {
    const root = await render({ deployments: [summary('default', 'stopped')] });

    const presets = root.queryAll(By.css('.settings__preset')).map(row => row.nativeElement.textContent.trim());
    expect(presets).toEqual([
      'local: Exasol Local deployment using a macOS VM.',
      'local: Uses the Exasol Local database container.',
    ]);
    expect(field(root, 'infrastructure-cpu-count').value).toBe('2');
    expect(field(root, 'infrastructure-cpu-count').type).toBe('number');
    expect(field(root, 'infrastructure-ports').value).toBe('db:8563');
    expect(field(root, 'infrastructure-ports').type).toBe('text');
    expect(box(root, 'Installation options').nativeElement.textContent).toContain(
      'The local installation preset has no options.'
    );
  });

  it('saves a changed option and offers to apply the changes', async () => {
    const root = await render({ deployments: [summary('default', 'stopped')] });
    api.setConfiguration.mockReturnValue(of(withOption('cpu-count', 4)));
    expect(root.query(By.css('.settings__apply'))).toBeNull();

    type(field(root, 'infrastructure-cpu-count'), '4');
    click(root, '.settings__save');
    await refresh();

    expect(api.setConfiguration).toHaveBeenCalledWith('default', { 'cpu-count': 4 });
    expect(field(root, 'infrastructure-cpu-count').value).toBe('4');
    expect(root.query(By.css('.settings__banner--apply')).nativeElement.textContent).toContain(
      'Saved changes take effect when you apply them to the deployment.'
    );

    click(root, '.settings__apply');
    expect(api.runOperation).toHaveBeenCalledWith('default', 'start');
  });

  it('resets the options of a preset after confirmation', async () => {
    const root = await render({
      deployments: [summary('default', 'stopped')],
      configurations: { default: withOption('cpu-count', 4) },
    });
    api.resetConfiguration.mockReturnValue(of(localConfiguration()));
    type(field(root, 'infrastructure-memory-mb'), '8192');

    click(root, '.settings__reset');
    await refresh();

    expect(TestBed.inject(MatDialog).open).toHaveBeenCalled();
    expect(api.resetConfiguration).toHaveBeenCalledWith('default', ['cpu-count', 'data-size-gb', 'memory-mb', 'ports']);
    expect(field(root, 'infrastructure-cpu-count').value).toBe('2');
    expect(field(root, 'infrastructure-memory-mb').value).toBe('24576');
  });

  it('keeps the options when the reset is cancelled', async () => {
    const root = await render({ deployments: [summary('default', 'stopped')] });
    confirmed = false;

    click(root, '.settings__reset');

    expect(api.resetConfiguration).not.toHaveBeenCalled();
  });

  it('shows the launcher message of a rejected change', async () => {
    const root = await render({ deployments: [summary('default', 'stopped')] });
    api.setConfiguration.mockReturnValue(
      throwError(() => new HttpErrorResponse({ status: 400, error: { detail: 'invalid number "-1"' } }))
    );

    type(field(root, 'infrastructure-cpu-count'), '-1');
    click(root, '.settings__save');
    await refresh();

    expect(
      box(root, 'Infrastructure options').query(By.css('.settings__save-error')).nativeElement.textContent
    ).toContain('invalid number "-1"');
    expect(field(root, 'infrastructure-cpu-count').value).toBe('-1');
    expect(root.query(By.css('.settings__apply'))).toBeNull();
  });

  it('shows the settings of a running local deployment read-only and says to stop it', async () => {
    const root = await render({
      deployments: [summary('default')],
      configurations: { default: localConfiguration({ editable: false, apply_with: undefined }) },
    });

    expect(field(root, 'infrastructure-cpu-count').disabled).toBe(true);
    expect(root.query(By.css('.settings__save'))).toBeNull();
    expect(root.query(By.css('.settings__reset'))).toBeNull();
    expect(root.query(By.css('.settings__read-only')).nativeElement.textContent).toContain(
      'Stop the deployment to change its settings. You can stop it on the Overview page.'
    );
  });

  it('says to destroy a deployed cloud deployment to change its settings', async () => {
    const root = await render(
      {
        deployments: [summary('aws', 'database_ready')],
        configurations: { aws: { ...awsConfiguration(), editable: false, apply_with: undefined } },
      },
      'aws'
    );

    expect(root.query(By.css('.settings__read-only')).nativeElement.textContent).toContain(
      'Destroy the deployment to change its settings.'
    );
  });

  it('offers Deploy for an initialized deployment and shows secret and boolean options', async () => {
    const root = await render(
      {
        deployments: [summary('aws', 'initialized')],
        configurations: { aws: awsConfiguration() },
      },
      'aws'
    );

    const password = field(root, 'infrastructure-db-password');
    expect(password.type).toBe('password');
    expect(password.disabled).toBe(true);
    expect(password.value).toBe('');
    expect(password.parentElement?.textContent).toContain('Set it with the launcher');

    const archive = box(root, 'Infrastructure options').query(By.css('input[type="checkbox"]'))
      .nativeElement as HTMLInputElement;
    expect(archive.checked).toBe(true);
    archive.click();
    click(box(root, 'Infrastructure options'), '.settings__save');
    expect(api.setConfiguration).toHaveBeenCalledWith('aws', { 's3-archive-enabled': false });

    expect(root.query(By.css('.settings__banner--deploy')).nativeElement.textContent).toContain(
      'This deployment has no resources yet. Deploy creates them with these settings.'
    );
    click(root, '.settings__deploy');
    expect(api.runOperation).toHaveBeenCalledWith('aws', 'deploy');
  });
});
