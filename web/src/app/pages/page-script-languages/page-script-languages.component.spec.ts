import { DebugElement } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { MatDialog } from '@angular/material/dialog';
import { By } from '@angular/platform-browser';
import { provideNoopAnimations } from '@angular/platform-browser/animations';
import { provideRouter } from '@angular/router';
import { Store, provideStore } from '@ngxs/store';
import { Subject, firstValueFrom, of } from 'rxjs';
import { Operation, ScriptLanguages } from '@exa/services/local-api/local-api.models';
import { LocalApiService } from '@exa/services/local-api/local-api.service';
import { OperationEvent, OperationEventsService } from '@exa/services/local-api/operation-events.service';
import { LoadDeploymentsAction } from '@exa/store/deployments/deployments.actions';
import { DeploymentsState } from '@exa/store/deployments/deployments.state';
import { LoadOperationsAction } from '@exa/store/operations/operations.actions';
import { OperationsState } from '@exa/store/operations/operations.state';
import { ScriptLanguagesState } from '@exa/store/script-languages/script-languages.state';
import {
  LocalApiFixture,
  defaultScriptLanguages,
  mockLocalApi,
  operation,
  summary,
} from '../../testing/local-api.mock';
import { PageScriptLanguagesComponent } from './page-script-languages.component';

describe('PageScriptLanguagesComponent', () => {
  let component: ComponentFixture<PageScriptLanguagesComponent>;
  let api: ReturnType<typeof mockLocalApi>;
  let dialogResult: unknown;
  let events: Subject<OperationEvent>;

  const refresh = async () => {
    component.detectChanges();
    await component.whenStable();
    component.detectChanges();
  };

  const render = async (fixture: LocalApiFixture, name = 'default'): Promise<DebugElement> => {
    api = mockLocalApi(fixture);
    dialogResult = undefined;
    events = new Subject<OperationEvent>();
    await TestBed.configureTestingModule({
      imports: [PageScriptLanguagesComponent],
      providers: [
        provideNoopAnimations(),
        provideRouter([]),
        provideStore([DeploymentsState, OperationsState, ScriptLanguagesState]),
        { provide: LocalApiService, useValue: api },
        { provide: MatDialog, useValue: { open: jest.fn(() => ({ afterClosed: () => of(dialogResult) })) } },
        { provide: OperationEventsService, useValue: { follow: jest.fn(() => events) } },
      ],
    }).compileComponents();

    await firstValueFrom(TestBed.inject(Store).dispatch(new LoadDeploymentsAction()));
    await firstValueFrom(TestBed.inject(Store).dispatch(new LoadOperationsAction()));
    component = TestBed.createComponent(PageScriptLanguagesComponent);
    component.componentRef.setInput('name', name);
    await refresh();
    return component.debugElement;
  };

  const text = (element: DebugElement, selector: string) =>
    (element.query(By.css(selector)).nativeElement.textContent as string).replace(/\s+/g, ' ').trim();
  const officialRows = (root: DebugElement) => root.queryAll(By.css('.script-languages__official'));
  const official = (root: DebugElement, language: string) =>
    officialRows(root).find(row => text(row, '.script-languages__language') === language)!;
  const customRows = (root: DebugElement) => root.queryAll(By.css('.script-languages__custom'));
  const click = (element: DebugElement, selector: string) => {
    const target = element.query(By.css(selector)).nativeElement as HTMLElement;
    (target.querySelector('button') ?? target).click();
  };
  const button = (element: DebugElement, selector: string): HTMLButtonElement => {
    const target = element.query(By.css(selector)).nativeElement as HTMLElement;
    return (target.querySelector('button') ?? target) as HTMLButtonElement;
  };
  const type = (root: DebugElement, field: string, value: string) => {
    const input = root.query(By.css(`#script-language-${field}`)).nativeElement as HTMLInputElement;
    input.value = value;
    input.dispatchEvent(new Event('input'));
  };
  const fieldValue = (root: DebugElement, field: string) =>
    (root.query(By.css(`#script-language-${field}`)).nativeElement as HTMLInputElement).value;
  const finish = async (finished: Operation, languages: ScriptLanguages) => {
    api.listScriptLanguages.mockReturnValue(of(languages));
    api.listOperations.mockReturnValue(of([finished]));
    events.next({ type: 'status', operation: finished });
    await refresh();
  };

  it('lists the official containers, Rust included, with their actions', async () => {
    const root = await render({ deployments: [summary('default')] });

    const headings = root
      .queryAll(By.css('.script-languages__table--official th'))
      .map(cell => cell.nativeElement.textContent.trim());
    expect(headings).toEqual(['Language', 'Flavor', 'Version', 'Aliases', 'State', 'Actions']);
    expect(officialRows(root).map(row => text(row, '.script-languages__language'))).toEqual([
      'java',
      'python',
      'r',
      'rust',
    ]);
    expect((root.query(By.css('.script-languages__restart input')).nativeElement as HTMLInputElement).checked).toBe(
      true
    );

    for (const installed of ['java', 'python']) {
      const row = official(root, installed);
      expect(text(row, '.script-languages__state')).toBe('installed');
      expect(row.query(By.css('.script-languages__update'))).toBeTruthy();
      expect(row.query(By.css('.script-languages__remove'))).toBeTruthy();
      expect(row.query(By.css('.script-languages__install'))).toBeNull();
    }
    for (const absent of ['r', 'rust']) {
      const row = official(root, absent);
      expect(text(row, '.script-languages__state')).toBe('not installed');
      expect(row.query(By.css('.script-languages__install'))).toBeTruthy();
      expect(row.query(By.css('.script-languages__remove'))).toBeNull();
    }
    expect(text(official(root, 'rust'), '.script-languages__flavor')).toBe('latest exasol-labs release');
    expect(text(official(root, 'java'), '.script-languages__flavor')).toBe('java-17');
  });

  it('installs a container with a restart and lists it as installed afterwards', async () => {
    const root = await render({ deployments: [summary('default')] });

    click(official(root, 'r'), '.script-languages__install');
    await refresh();

    expect(api.runOperation).toHaveBeenCalledWith('default', 'install_script_language', {
      script_language: 'R',
      restart: true,
    });
    events.next({ type: 'line', line: 'Pulling the image' });
    await refresh();
    expect(text(root, '.operation-progress__text')).toBe('Installing R on default.');
    expect(text(root, '.operation-progress__log')).toBe('Pulling the image');
    expect(button(official(root, 'java'), '.script-languages__update').disabled).toBe(true);
    expect(button(root, '.script-languages__install-custom').disabled).toBe(true);

    const installed = defaultScriptLanguages();
    installed.official[2].installed = true;
    await finish(
      operation({ id: 'new', kind: 'install_script_language', script_language: 'R', status: 'succeeded' }),
      installed
    );

    expect(root.query(By.css('.operation-progress'))).toBeNull();
    expect(text(official(root, 'r'), '.script-languages__state')).toBe('installed');
    expect(button(official(root, 'r'), '.script-languages__update').disabled).toBe(false);
  });

  it('updates a container', async () => {
    const root = await render({ deployments: [summary('default')] });

    click(official(root, 'python'), '.script-languages__update');
    await refresh();

    expect(api.runOperation).toHaveBeenCalledWith('default', 'update_script_language', {
      script_language: 'PYTHON3',
      restart: true,
    });
  });

  it('removes a container for the next start after confirmation', async () => {
    const root = await render({ deployments: [summary('default')] });
    dialogResult = true;

    const restart = root.query(By.css('.script-languages__restart input')).nativeElement as HTMLInputElement;
    restart.click();
    await refresh();
    click(official(root, 'java'), '.script-languages__remove');
    await refresh();

    expect(TestBed.inject(MatDialog).open).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({
        data: expect.objectContaining({
          title: 'Remove JAVA?',
          content:
            'This removes the script language container JAVA from default. ' +
            'The change applies on the next start of default.',
        }),
      })
    );
    expect(api.runOperation).toHaveBeenCalledWith('default', 'remove_script_language', {
      script_language: 'JAVA',
      restart: false,
    });

    const removed = defaultScriptLanguages();
    removed.official[0].installed = false;
    await finish(
      operation({ id: 'new', kind: 'remove_script_language', script_language: 'JAVA', status: 'succeeded' }),
      removed
    );
    expect(text(official(root, 'java'), '.script-languages__state')).toBe('not installed');
  });

  it('names the restart when confirming a removal', async () => {
    const root = await render({ deployments: [summary('default')] });

    click(customRows(root)[0], '.script-languages__remove');
    await refresh();

    expect(TestBed.inject(MatDialog).open).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({
        data: expect.objectContaining({
          content:
            'This removes the script language container MYPY3 from default. ' +
            'The database restarts to apply the change.',
        }),
      })
    );
    expect(api.runOperation).not.toHaveBeenCalled();
  });

  it('lists the custom containers with their source and state', async () => {
    const languages = defaultScriptLanguages();
    languages.custom.push({ alias: 'OLDPY', language: 'python', source: '/gone.tar.gz', available: false });
    const root = await render({ deployments: [summary('default')], scriptLanguages: { default: languages } });

    const headings = root
      .queryAll(By.css('.script-languages__table--custom th'))
      .map(cell => cell.nativeElement.textContent.trim());
    expect(headings).toEqual(['Alias', 'Language', 'Source', 'State', 'Actions']);
    const rows = customRows(root);
    expect(rows.map(row => text(row, '.script-languages__alias'))).toEqual(['MYPY3', 'OLDPY']);
    expect(text(rows[0], '.script-languages__source')).toBe('/Users/me/containers/my-python.tar.gz');
    expect(text(rows[0], '.script-languages__state')).toBe('installed');
    expect(text(rows[1], '.script-languages__state')).toBe('unavailable');
  });

  it('installs a custom container and clears the form once it succeeds', async () => {
    const languages = defaultScriptLanguages();
    languages.custom = [];
    const root = await render({ deployments: [summary('default')], scriptLanguages: { default: languages } });
    expect(root.query(By.css('.script-languages__table--custom'))).toBeNull();
    expect(button(root, '.script-languages__install-custom').disabled).toBe(true);

    type(root, 'alias', 'MYPY3');
    type(root, 'language', 'python');
    type(root, 'source', ' /Users/me/containers/my-python.tar.gz ');
    await refresh();
    click(root, '.script-languages__install-custom');
    await refresh();

    expect(api.runOperation).toHaveBeenCalledWith('default', 'install_custom_script_language', {
      script_language: 'MYPY3',
      language: 'python',
      source: '/Users/me/containers/my-python.tar.gz',
      restart: true,
    });
    expect(text(root, '.operation-progress__text')).toBe('Installing MYPY3 on default.');

    await finish(
      operation({ id: 'new', kind: 'install_custom_script_language', script_language: 'MYPY3', status: 'succeeded' }),
      defaultScriptLanguages()
    );

    expect(customRows(root).map(row => text(row, '.script-languages__alias'))).toEqual(['MYPY3']);
    expect(['alias', 'language', 'source'].map(field => fieldValue(root, field))).toEqual(['', '', '']);
  });

  it('updates an existing custom container', async () => {
    const root = await render({ deployments: [summary('default')] });

    type(root, 'alias', 'MYPY3');
    type(root, 'language', 'python');
    type(root, 'source', 'https://example.com/my-python-2.tar.gz');
    await refresh();
    click(root, '.script-languages__update-custom');
    await refresh();

    expect(api.runOperation).toHaveBeenCalledWith('default', 'update_custom_script_language', {
      script_language: 'MYPY3',
      language: 'python',
      source: 'https://example.com/my-python-2.tar.gz',
      restart: true,
    });
  });

  it('reports a refused custom container and keeps the entered values', async () => {
    const root = await render({ deployments: [summary('default')] });

    type(root, 'alias', 'MYPY3');
    type(root, 'language', 'python');
    type(root, 'source', '/tmp/not-a-container.tar.gz');
    await refresh();
    click(root, '.script-languages__install-custom');
    await refresh();

    await finish(
      operation({
        id: 'new',
        kind: 'install_custom_script_language',
        script_language: 'MYPY3',
        status: 'failed',
        error: 'the archive is no script language container',
      }),
      defaultScriptLanguages()
    );

    expect(text(root, '.script-languages__failure')).toBe(
      'Installing MYPY3 on default failed: the archive is no script language container'
    );
    expect(['alias', 'language', 'source'].map(field => fieldValue(root, field))).toEqual([
      'MYPY3',
      'python',
      '/tmp/not-a-container.tar.gz',
    ]);
  });

  it('shows the message for deployments without script language containers', async () => {
    const root = await render(
      {
        deployments: [summary('aws')],
        scriptLanguageRefusals: {
          aws: 'script language container management is only supported for local deployments',
        },
      },
      'aws'
    );

    expect(text(root, '.script-languages__message--error')).toBe(
      'script language container management is only supported for local deployments'
    );
    expect(root.query(By.css('.script-languages__table'))).toBeNull();
  });
});
