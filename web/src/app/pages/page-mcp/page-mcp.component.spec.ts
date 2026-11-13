import { HttpErrorResponse } from '@angular/common/http';
import { DebugElement } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { MatDialog } from '@angular/material/dialog';
import { By } from '@angular/platform-browser';
import { provideNoopAnimations } from '@angular/platform-browser/animations';
import { provideRouter } from '@angular/router';
import { Store, provideStore } from '@ngxs/store';
import { Subject, firstValueFrom, of, throwError } from 'rxjs';
import { LocalApiService } from '@exa/services/local-api/local-api.service';
import { OperationEvent, OperationEventsService } from '@exa/services/local-api/operation-events.service';
import { LoadDeploymentsAction } from '@exa/store/deployments/deployments.actions';
import { DeploymentsState } from '@exa/store/deployments/deployments.state';
import { LoadOperationsAction } from '@exa/store/operations/operations.actions';
import { OperationsState } from '@exa/store/operations/operations.state';
import { SidecarsState } from '@exa/store/sidecars/sidecars.state';
import { LocalApiFixture, mockLocalApi, operation, sidecar, summary } from '../../testing/local-api.mock';
import { PageMcpComponent } from './page-mcp.component';

const disabledMcp = () => sidecar({ enabled: false, running: false, state: '', health: '', endpoints: [] });

describe('PageMcpComponent', () => {
  let component: ComponentFixture<PageMcpComponent>;
  let api: ReturnType<typeof mockLocalApi>;
  let dialogResult: unknown;
  let events: Subject<OperationEvent>;
  let writeText: jest.Mock;

  const refresh = async () => {
    component.detectChanges();
    await component.whenStable();
    component.detectChanges();
  };

  const render = async (fixture: LocalApiFixture, name = 'default'): Promise<DebugElement> => {
    api = mockLocalApi(fixture);
    dialogResult = undefined;
    events = new Subject<OperationEvent>();
    writeText = jest.fn(() => Promise.resolve());
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });
    await TestBed.configureTestingModule({
      imports: [PageMcpComponent],
      providers: [
        provideNoopAnimations(),
        provideRouter([]),
        provideStore([DeploymentsState, OperationsState, SidecarsState]),
        { provide: LocalApiService, useValue: api },
        { provide: MatDialog, useValue: { open: jest.fn(() => ({ afterClosed: () => of(dialogResult) })) } },
        { provide: OperationEventsService, useValue: { follow: jest.fn(() => events) } },
      ],
    }).compileComponents();

    await firstValueFrom(TestBed.inject(Store).dispatch(new LoadDeploymentsAction()));
    await firstValueFrom(TestBed.inject(Store).dispatch(new LoadOperationsAction()));
    component = TestBed.createComponent(PageMcpComponent);
    component.componentRef.setInput('name', name);
    await refresh();
    return component.debugElement;
  };

  const text = (root: DebugElement, selector: string) =>
    (root.query(By.css(selector)).nativeElement.textContent as string).replace(/\s+/g, ' ').trim();
  const box = (root: DebugElement, label: string) => root.query(By.css(`.mcp__box[data-label="${label}"]`));
  const choose = async (root: DebugElement, client: string) => {
    const select = root.query(By.css('#mcp-client')).nativeElement as HTMLSelectElement;
    select.value = client;
    select.dispatchEvent(new Event('change'));
    await refresh();
  };

  it('shows the MCP server with its state, health, and description', async () => {
    const root = await render({ deployments: [summary('default')] });

    expect(text(root, '.mcp__state')).toBe('running');
    expect(text(root, '.mcp__title')).toBe('MCP server');
    expect(text(root, '.mcp__description')).toBe('Exasol MCP server with read-oriented query tools');
    expect(text(root, '.mcp__health')).toBe('healthy');
    expect(root.query(By.css('.mcp__disable'))).toBeTruthy();
    expect(root.query(By.css('.mcp__enable'))).toBeNull();
  });

  it('shows the endpoint with a copy button', async () => {
    const root = await render({ deployments: [summary('default')] });

    expect(text(box(root, 'Endpoint'), '.mcp__endpoint')).toBe('http://127.0.0.1:4896/mcp');
    box(root, 'Endpoint').query(By.css('button[aria-label="Copy the endpoint"]')).nativeElement.click();

    expect(writeText).toHaveBeenCalledWith('http://127.0.0.1:4896/mcp');
  });

  it('shows the registration command of the first client and copies it', async () => {
    const root = await render({ deployments: [summary('default')] });

    expect(api.getMcpInstructions).toHaveBeenCalledWith('default', 'claude-code');
    const client = box(root, 'Connect a client');
    expect(text(client, '.mcp__lead')).toBe('Run this command to register the MCP server with Claude Code:');
    const command = 'claude mcp add --transport http --scope user exasol-personal http://127.0.0.1:4896/mcp';
    expect(text(client, '.mcp__command')).toBe(command);

    client.query(By.css('button[aria-label="Copy the command"]')).nativeElement.click();
    expect(writeText).toHaveBeenCalledWith(command);
    expect(text(client, '.mcp__note')).toContain("Exasol Local does not change the client's configuration.");
  });

  it('names the configuration file and shows the entry for a client configured through a file', async () => {
    const root = await render({ deployments: [summary('default')] });

    await choose(root, 'cursor');

    expect(api.getMcpInstructions).toHaveBeenLastCalledWith('default', 'cursor');
    const client = box(root, 'Connect a client');
    expect(text(client, '.mcp__lead')).toBe(
      'Add this entry to ~/.cursor/mcp.json for Cursor, keeping any servers already there:'
    );
    expect(client.query(By.css('.mcp__content')).nativeElement.textContent).toContain(
      '"url": "http://127.0.0.1:4896/mcp"'
    );
    expect(client.query(By.css('.mcp__command'))).toBeNull();

    client.query(By.css('button[aria-label="Copy the entry"]')).nativeElement.click();
    expect(writeText).toHaveBeenCalledWith(expect.stringContaining('"mcpServers"'));
  });

  it('reports a failing health check below the endpoint', async () => {
    const root = await render({
      deployments: [summary('default')],
      sidecars: {
        default: [
          sidecar({
            health: 'failing',
            health_error: 'Get "http://127.0.0.1:4896/health": context deadline exceeded',
          }),
        ],
      },
    });

    expect(text(root, '.mcp__health')).toBe('health failing');
    expect(text(box(root, 'Endpoint'), '.mcp__health-error')).toBe(
      'Health check failed: Get "http://127.0.0.1:4896/health": context deadline exceeded'
    );
  });

  it('enables a disabled MCP server and then shows the endpoint and the client instructions', async () => {
    const root = await render({ deployments: [summary('default')], sidecars: { default: [disabledMcp()] } });

    expect(text(root, '.mcp__state')).toBe('disabled');
    expect(text(root, '.mcp__panel')).toBe('Enable the MCP server to connect AI clients to default.');
    expect(api.listMcpClients).not.toHaveBeenCalled();

    root.query(By.css('.mcp__enable button')).nativeElement.click();
    await refresh();
    expect(api.runOperation).toHaveBeenCalledWith('default', 'enable_sidecar', 'mcp');
    events.next({ type: 'line', line: 'Waiting for the MCP server' });
    await refresh();
    expect(text(root, '.operation-progress__log')).toBe('Waiting for the MCP server');

    api.listSidecars.mockReturnValue(of([sidecar()]));
    const finished = operation({ id: 'new', kind: 'enable_sidecar', sidecar: 'mcp', status: 'succeeded' });
    api.listOperations.mockReturnValue(of([finished]));
    events.next({ type: 'status', operation: finished });
    await refresh();

    expect(root.query(By.css('.operation-progress'))).toBeNull();
    expect(text(box(root, 'Endpoint'), '.mcp__endpoint')).toBe('http://127.0.0.1:4896/mcp');
    expect(text(box(root, 'Connect a client'), '.mcp__command')).toContain('claude mcp add');
  });

  it('disables the MCP server after confirmation', async () => {
    const root = await render({ deployments: [summary('default')] });
    dialogResult = 'disable_sidecar';

    root.query(By.css('.mcp__disable')).nativeElement.click();
    await refresh();

    expect(api.runOperation).toHaveBeenCalledWith('default', 'disable_sidecar', 'mcp');
  });

  it('shows why the launcher refuses the client instructions', async () => {
    const root = await render({ deployments: [summary('default')] });
    api.getMcpInstructions.mockReturnValue(
      throwError(
        () =>
          new HttpErrorResponse({
            status: 409,
            error: { title: 'Conflict', detail: 'the MCP server is not enabled for this deployment' },
          })
      )
    );

    await choose(root, 'claude-desktop');

    expect(text(box(root, 'Connect a client'), '.mcp__message--error')).toBe(
      'the MCP server is not enabled for this deployment'
    );
  });

  it('shows the launcher message for deployments without sidecars', async () => {
    const root = await render(
      {
        deployments: [summary('aws')],
        sidecarRefusals: { aws: 'sidecars are not supported for this deployment type' },
      },
      'aws'
    );

    expect(text(root, '.mcp__message--error')).toBe('sidecars are not supported for this deployment type');
    expect(root.query(By.css('.mcp__card'))).toBeNull();
  });
});
