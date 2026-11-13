import { DebugElement } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { provideNoopAnimations } from '@angular/platform-browser/animations';
import { Router, provideRouter } from '@angular/router';
import { Store, provideStore } from '@ngxs/store';
import { Subject, firstValueFrom } from 'rxjs';
import { LocalApiService } from '@exa/services/local-api/local-api.service';
import { OperationEventsService } from '@exa/services/local-api/operation-events.service';
import { loadText, saveText } from '@exa/sql/storage';
import { LoadDeploymentsAction } from '@exa/store/deployments/deployments.actions';
import { DeploymentsState } from '@exa/store/deployments/deployments.state';
import { ImportsState } from '@exa/store/imports/imports.state';
import { LoadOperationsAction } from '@exa/store/operations/operations.actions';
import { OperationsState } from '@exa/store/operations/operations.state';
import { LocalApiFixture, dataImport, mockLocalApi, summary, tripsSuggestion } from '../../testing/local-api.mock';
import { PageImportComponent } from './page-import.component';

describe('PageImportComponent', () => {
  let component: ComponentFixture<PageImportComponent>;
  let api: ReturnType<typeof mockLocalApi>;
  let root: DebugElement;

  const refresh = async () => {
    component.detectChanges();
    await component.whenStable();
    component.detectChanges();
  };

  const render = async (fixture: Partial<LocalApiFixture> = {}, name = 'default') => {
    api = mockLocalApi({
      deployments: [summary('default'), summary('test', 'stopped')],
      suggestions: { 'upload-1': tripsSuggestion() },
      ...fixture,
    });
    await TestBed.configureTestingModule({
      imports: [PageImportComponent],
      providers: [
        provideNoopAnimations(),
        provideRouter([]),
        provideStore([DeploymentsState, OperationsState, ImportsState]),
        { provide: LocalApiService, useValue: api },
        { provide: OperationEventsService, useValue: { follow: jest.fn(() => new Subject()) } },
      ],
    }).compileComponents();

    await firstValueFrom(TestBed.inject(Store).dispatch(new LoadDeploymentsAction()));
    await firstValueFrom(TestBed.inject(Store).dispatch(new LoadOperationsAction()));
    component = TestBed.createComponent(PageImportComponent);
    component.componentRef.setInput('name', name);
    root = component.debugElement;
    await refresh();
  };

  const text = (selector: string, within = root) =>
    (within.query(By.css(selector)).nativeElement.textContent as string).replace(/\s+/g, ' ').trim();
  const button = (selector: string): HTMLButtonElement => {
    const target = root.query(By.css(selector)).nativeElement as HTMLElement;
    return (target.querySelector('button') ?? target) as HTMLButtonElement;
  };
  const parquet = (name: string) => new File([new Uint8Array(4)], name);

  const drop = async (...files: File[]) => {
    const event = new Event('drop', { cancelable: true }) as DragEvent;
    Object.defineProperty(event, 'dataTransfer', { value: { files } });
    root.query(By.css('.import__drop')).nativeElement.dispatchEvent(event);
    await refresh();
  };

  beforeEach(() => localStorage.clear());

  it('uploads dropped files with their progress', async () => {
    await render();
    const sent = [new Subject<number>(), new Subject<number>()];
    api.uploadFile.mockImplementation(() => sent[api.uploadFile.mock.calls.length - 1]);

    await drop(parquet('yellow_tripdata_2024-01.parquet'), parquet('yellow_tripdata_2024-02.parquet'));
    sent[0].next(0.5);
    await refresh();

    const uploads = root.queryAll(By.css('.import__upload'));
    expect(uploads.map(upload => [text('.import__file-name', upload), text('.import__meta', upload)])).toEqual([
      ['yellow_tripdata_2024-01.parquet', '50 %'],
      ['yellow_tripdata_2024-02.parquet', '0 %'],
    ]);
    expect(button('.import__choose').disabled).toBe(true);

    sent[0].next(1);
    sent[0].complete();
    sent[1].next(1);
    sent[1].complete();
    await refresh();

    expect(api.suggestImport).toHaveBeenCalledWith('default', { kind: 'upload', upload_id: 'upload-1' });
    expect(root.query(By.css('.import__drop'))).toBeNull();
  });

  it('uploads chosen files', async () => {
    await render();
    const input = root.query(By.css('.import__file-input')).nativeElement as HTMLInputElement;
    Object.defineProperty(input, 'files', { value: [parquet('trips.parquet')], configurable: true });

    input.dispatchEvent(new Event('change'));
    await refresh();

    expect(api.createUpload).toHaveBeenCalledWith('default', [{ name: 'trips.parquet', size: 4 }]);
  });

  it('names files that cannot be read next to the drop area', async () => {
    await render({ suggestions: {} });

    await drop(parquet('notes.parquet'));

    expect(text('.import__note--error')).toBe('notes.parquet is not a Parquet file, or it is damaged.');
    expect(root.query(By.css('.import__drop'))).toBeTruthy();
  });

  it('lists recent imports with a failed import’s message', async () => {
    await render({
      imports: {
        default: [
          dataImport({ id: 'a', status: 'succeeded', rows: 5_972_150, seconds: 13.4 }),
          dataImport({
            id: 'b',
            table: 'ORDERS',
            status: 'failed',
            seconds: 3.1,
            source: { kind: 'upload', label: 'orders.parquet', files: 1, bytes: 10 },
            message: 'ETL-3076: [Column=order_total] value out of range',
          }),
        ],
      },
    });

    const rows = root.queryAll(By.css('.import__recent tbody tr'));
    const cells = (row: DebugElement) =>
      row
        .queryAll(By.css('td'))
        .map(cell => cell.nativeElement.textContent.trim())
        .filter(Boolean)
        .join(' ');
    expect(rows.map(cells)).toEqual([
      'DATA.YELLOW_TRIPDATA_2024 2 files 5,972,150 13.4 s succeeded Query',
      'DATA.ORDERS orders.parquet 3.1 s failed',
      'ETL-3076: [Column=order_total] value out of range',
    ]);
  });

  it('queries an imported table in the SQL editor', async () => {
    await render({ imports: { default: [dataImport({ status: 'succeeded', rows: 1 })] } });
    saveText('default', 'SELECT 1;');
    const navigate = jest.spyOn(TestBed.inject(Router), 'navigate').mockResolvedValue(true);

    button('.import__query').click();

    expect(loadText('default')).toBe('SELECT 1;\nSELECT * FROM "DATA"."YELLOW_TRIPDATA_2024" LIMIT 100;');
    expect(navigate).toHaveBeenCalledWith(['/', 'deployments', 'default', 'sql']);
  });

  it('offers Start and disables choosing files while the database does not run', async () => {
    await render({ imports: { test: [dataImport({ status: 'succeeded', rows: 1 })] } }, 'test');

    expect(text('.import__banner')).toContain('Imports need a running database');
    expect(button('.import__choose').disabled).toBe(true);
    expect(root.queryAll(By.css('.import__entry'))).toHaveLength(1);

    button('.import__start').click();

    expect(api.runOperation).toHaveBeenCalledWith('test', 'start');
  });

  describe('suggested table', () => {
    const input = (selector: string): HTMLInputElement => root.query(By.css(selector)).nativeElement;
    const type = async (selector: string, value: string) => {
      const field = input(selector);
      field.value = value;
      field.dispatchEvent(new Event('input'));
      await refresh();
    };
    const columnRows = () => root.queryAll(By.css('.import__column'));
    const importButton = () => button('.import__start-import');

    const suggest = async (suggestion = tripsSuggestion()) => {
      await render({ suggestions: { 'upload-1': suggestion } });
      await drop(parquet('yellow_tripdata_2024-01.parquet'), parquet('yellow_tripdata_2024-02.parquet'));
    };

    it('shows the files, the table, and the columns with types and samples', async () => {
      await suggest();

      expect(text('.import__file-name')).toBe('yellow_tripdata_2024-01.parquet, yellow_tripdata_2024-02.parquet');
      expect(text('.import__source-summary')).toBe('2 files · 98.4 MB · 5,972,150 rows');
      expect(input('#import-schema').value).toBe('DATA');
      expect(input('#import-table').value).toBe('YELLOW_TRIPDATA_2024');
      expect(
        columnRows().map(row => [
          (row.query(By.css('.import__cell-name')).nativeElement as HTMLInputElement).value,
          (row.query(By.css('.import__cell-type')).nativeElement as HTMLInputElement).value,
          text('td:nth-child(4)', row),
          text('.import__sample', row),
        ])
      ).toEqual([
        ['VendorID', 'DECIMAL(10,0)', 'INT32', '2'],
        ['passenger_count', 'DECIMAL(20,0)', 'INT64', '1'],
        ['trip_distance', 'DOUBLE PRECISION', 'DOUBLE', '1.72'],
      ]);
      expect(text('.import__start-import')).toBe('Import 5,972,150 rows');
      expect(importButton().disabled).toBe(false);
    });

    it('imports with the changed names and types, and resets them to the suggestion', async () => {
      await suggest();

      await type('.import__column:nth-child(2) .import__cell-name', 'PASSENGERS');
      await type('.import__column:nth-child(2) .import__cell-type', 'DECIMAL(3,0)');

      expect(input('.import__column:nth-child(2) .import__cell-type').classList).toContain('import__cell--changed');

      await root.query(By.css('.import__reset')).nativeElement.click();
      await refresh();

      expect(input('.import__column:nth-child(2) .import__cell-name').value).toBe('passenger_count');

      await type('#import-table', 'TRIPS');
      await type('.import__column:nth-child(2) .import__cell-type', 'DECIMAL(3,0)');
      importButton().click();
      await refresh();

      expect(api.startImport).toHaveBeenCalledWith('default', {
        source: { kind: 'upload', upload_id: 'upload-1' },
        schema: 'DATA',
        table: 'TRIPS',
        columns: [
          { name: 'VendorID', type: 'DECIMAL(10,0)' },
          { name: 'passenger_count', type: 'DECIMAL(3,0)' },
          { name: 'trip_distance', type: 'DOUBLE PRECISION' },
        ],
      });
    });

    it('disables the import while the table or a column name is empty', async () => {
      await suggest();

      await type('.import__column:nth-child(1) .import__cell-name', '');
      expect(importButton().disabled).toBe(true);

      await type('.import__column:nth-child(1) .import__cell-name', 'VENDOR');
      await type('#import-table', '');
      expect(importButton().disabled).toBe(true);
    });

    it('uses the alternative to a taken table name', async () => {
      await suggest(
        tripsSuggestion({
          table: 'EVENTS',
          importable: false,
          problems: [
            {
              code: 'table_taken',
              message: 'DATA.EVENTS exists. Choose another name, such as EVENTS_2.',
              alternative: 'EVENTS_2',
            },
          ],
        })
      );

      expect(text('.import__problem')).toBe('DATA.EVENTS exists. Choose another name, such as EVENTS_2. Use EVENTS_2');
      expect(input('#import-table').classList).toContain('import__input--bad');
      expect(importButton().disabled).toBe(true);

      root.query(By.css('.import__alternative')).nativeElement.click();
      await refresh();

      expect(input('#import-table').value).toBe('EVENTS_2');
      expect(root.query(By.css('.import__problem'))).toBeNull();
      expect(importButton().disabled).toBe(false);
    });

    it('blocks the import of a nested column', async () => {
      const message =
        'Exasol cannot import the column payload (struct): it does not support nested types, such as structs and ' +
        'lists. Remove it from the file, or import other files.';
      await suggest(
        tripsSuggestion({
          columns: [
            { name: 'id', type: 'DECIMAL(20,0)', parquet_type: 'INT64', sample: '7' },
            { name: 'payload', type: null, parquet_type: 'STRUCT', sample: null },
          ],
          importable: false,
          problems: [{ code: 'unsupported_columns', message, columns: ['payload'] }],
        })
      );

      expect(text('.import__problem')).toBe(message);
      expect(text('.import__column:nth-child(2) .import__unsupported')).toBe('Cannot be imported');
      expect(importButton().disabled).toBe(true);
    });

    it('shows the first ten columns until asked for all', async () => {
      const columns = Array.from({ length: 19 }, (_, index) => ({
        name: `c${index}`,
        type: 'DECIMAL(10,0)',
        parquet_type: 'INT32',
        sample: null,
      }));
      await suggest(tripsSuggestion({ columns }));

      expect(columnRows()).toHaveLength(10);
      expect(text('.import__show-all')).toBe('Show all 19 columns');

      root.query(By.css('.import__show-all')).nativeElement.click();
      await refresh();

      expect(columnRows()).toHaveLength(19);
      expect(root.query(By.css('.import__show-all'))).toBeNull();
    });

    it('returns to the drop area for other files', async () => {
      await suggest();

      root.query(By.css('.import__other-files')).nativeElement.click();
      await refresh();

      expect(root.query(By.css('.import__drop'))).toBeTruthy();
    });
  });
});
