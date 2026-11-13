import { TestBed } from '@angular/core/testing';
import { Store, provideStore } from '@ngxs/store';
import { firstValueFrom, of } from 'rxjs';
import { DataImport } from '@exa/services/local-api/local-api.models';
import { LocalApiService } from '@exa/services/local-api/local-api.service';
import { LocalApiFixture, dataImport, mockLocalApi, tripsSuggestion } from '../../testing/local-api.mock';
import {
  CancelImportAction,
  ChooseFilesAgainAction,
  EditImportColumnAction,
  EditImportTableAction,
  LoadImportsAction,
  RefreshImportAction,
  ResetImportAction,
  StartImportAction,
  UploadFilesAction,
} from './imports.actions';
import { ImportPage, ImportsState, canImport, openProblems } from './imports.state';

const parquet = (name: string, size = 4) => new File([new Uint8Array(size)], name);

const takenSuggestion = () =>
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
  });

describe('ImportsState', () => {
  let store: Store;
  let api: ReturnType<typeof mockLocalApi>;

  const setUp = (fixture: Partial<LocalApiFixture> = {}) => {
    api = mockLocalApi({ deployments: [], suggestions: { 'upload-1': tripsSuggestion() }, ...fixture });
    TestBed.configureTestingModule({
      providers: [provideStore([ImportsState]), { provide: LocalApiService, useValue: api }],
    });
    store = TestBed.inject(Store);
  };

  const page = (name = 'default'): ImportPage => store.selectSnapshot(ImportsState.pages)[name];
  const dispatch = (action: object) => firstValueFrom(store.dispatch(action));
  const uploadTrips = () =>
    dispatch(
      new UploadFilesAction('default', [
        parquet('yellow_tripdata_2024-01.parquet'),
        parquet('yellow_tripdata_2024-02.parquet', 6),
      ])
    );

  it('uploads files and shows the suggested table for them', async () => {
    setUp();
    const progress: number[][] = [];
    api.uploadFile.mockImplementation(() => {
      progress.push(page().uploads.map(upload => upload.sent));
      return of(0.5, 1);
    });

    await uploadTrips();

    expect(api.createUpload).toHaveBeenCalledWith('default', [
      { name: 'yellow_tripdata_2024-01.parquet', size: 4 },
      { name: 'yellow_tripdata_2024-02.parquet', size: 6 },
    ]);
    expect(api.uploadFile.mock.calls.map(([url]) => url)).toEqual([
      '/api/v1/deployments/default/uploads/upload-1/files/0',
      '/api/v1/deployments/default/uploads/upload-1/files/1',
    ]);
    expect(progress).toEqual([
      [0, 0],
      [1, 0],
    ]);
    expect(api.suggestImport).toHaveBeenCalledWith('default', { kind: 'upload', upload_id: 'upload-1' });
    expect(page().phase).toBe('suggestion');
    expect(page().draft).toEqual({
      schema: 'DATA',
      table: 'YELLOW_TRIPDATA_2024',
      columns: [
        { name: 'VendorID', type: 'DECIMAL(10,0)' },
        { name: 'passenger_count', type: 'DECIMAL(20,0)' },
        { name: 'trip_distance', type: 'DOUBLE PRECISION' },
      ],
    });
    expect(canImport(page())).toBe(true);
  });

  it('shows why files cannot be imported', async () => {
    setUp({ suggestions: {} });

    await dispatch(new UploadFilesAction('default', [parquet('notes.parquet')]));

    expect(page().phase).toBe('choose');
    expect(page().fileError).toBe('notes.parquet is not a Parquet file, or it is damaged.');
  });

  it('resolves a taken table name with its alternative', async () => {
    setUp({ suggestions: { 'upload-1': takenSuggestion() } });
    await uploadTrips();

    expect(openProblems(page()).map(problem => problem.code)).toEqual(['table_taken']);
    expect(canImport(page())).toBe(false);

    await dispatch(new EditImportTableAction('default', { table: 'EVENTS_2' }));

    expect(openProblems(page())).toEqual([]);
    expect(canImport(page())).toBe(true);
  });

  it('keeps the import disabled while a column cannot be imported', async () => {
    const suggestion = tripsSuggestion({
      columns: [
        { name: 'id', type: 'DECIMAL(20,0)', parquet_type: 'INT64', sample: '1' },
        { name: 'payload', type: null, parquet_type: 'STRUCT', sample: null },
      ],
      importable: false,
      problems: [{ code: 'unsupported_columns', message: 'Exasol cannot import…', columns: ['payload'] }],
    });
    setUp({ suggestions: { 'upload-1': suggestion } });
    await uploadTrips();

    await dispatch(new EditImportColumnAction('default', 1, { type: 'VARCHAR(100)' }));

    expect(canImport(page())).toBe(false);
  });

  it('needs a schema, a table, and every column name', async () => {
    setUp();
    await uploadTrips();

    await dispatch(new EditImportColumnAction('default', 0, { name: ' ' }));
    expect(canImport(page())).toBe(false);

    await dispatch(new ResetImportAction('default'));
    expect(canImport(page())).toBe(true);

    await dispatch(new EditImportTableAction('default', { schema: '' }));
    expect(canImport(page())).toBe(false);
  });

  it('starts the import with the suggested columns unless they changed', async () => {
    setUp();
    await uploadTrips();

    await dispatch(new StartImportAction('default'));

    expect(api.startImport).toHaveBeenLastCalledWith('default', {
      source: { kind: 'upload', upload_id: 'upload-1' },
      schema: 'DATA',
      table: 'YELLOW_TRIPDATA_2024',
      columns: undefined,
    });
    expect(page().phase).toBe('running');
    expect(page().recent.map(entry => entry.id)).toEqual(['import-1']);

    await dispatch(new ChooseFilesAgainAction('default'));
    await uploadTrips();
    await dispatch(new EditImportColumnAction('default', 1, { name: 'PASSENGERS', type: 'DECIMAL(3,0)' }));
    await dispatch(new StartImportAction('default'));

    expect(api.startImport.mock.lastCall?.[1].columns).toEqual([
      { name: 'VendorID', type: 'DECIMAL(10,0)' },
      { name: 'PASSENGERS', type: 'DECIMAL(3,0)' },
      { name: 'trip_distance', type: 'DOUBLE PRECISION' },
    ]);
  });

  const runImport = async (finished: DataImport) => {
    setUp({ imports: { default: [finished] } });
    await uploadTrips();
    await dispatch(new EditImportTableAction('default', { table: 'TRIPS' }));
    await dispatch(new StartImportAction('default'));
    await dispatch(new RefreshImportAction('default'));
  };

  it('shows a succeeded import', async () => {
    await runImport(dataImport({ status: 'succeeded', rows: 5_972_150 }));

    expect(page().phase).toBe('done');
    expect(page().current?.rows).toBe(5_972_150);
    expect(page().recent).toHaveLength(1);
  });

  it('returns to the edited suggestion after a failed import', async () => {
    await runImport(dataImport({ status: 'failed', message: 'value out of range' }));

    expect(page().phase).toBe('suggestion');
    expect(page().importError).toBe('value out of range');
    expect(page().draft?.table).toBe('TRIPS');
  });

  it('cancels the running import', async () => {
    setUp({ imports: { default: [dataImport({ status: 'cancelled' })] } });
    await uploadTrips();
    await dispatch(new StartImportAction('default'));

    await dispatch(new CancelImportAction('default'));

    expect(api.cancelImport).toHaveBeenCalledWith('default', 'import-1');
    expect(page().phase).toBe('suggestion');
    expect(page().importError).toBeUndefined();
  });

  it('finds a running import when the page opens', async () => {
    setUp({ imports: { default: [dataImport(), dataImport({ id: 'older', status: 'succeeded' })] } });

    await dispatch(new LoadImportsAction('default'));

    expect(page().phase).toBe('running');
    expect(page().current?.id).toBe('import-1');
    expect(page().recent.map(entry => entry.id)).toEqual(['import-1', 'older']);
  });

  it('keeps the pages of deployments apart', async () => {
    setUp();
    await uploadTrips();

    await dispatch(new LoadImportsAction('test'));

    expect(page('test').phase).toBe('choose');
    expect(page('default').phase).toBe('suggestion');
  });
});
