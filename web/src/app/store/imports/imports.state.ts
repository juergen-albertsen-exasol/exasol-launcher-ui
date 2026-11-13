import { Injectable, inject } from '@angular/core';
import { Action, Selector, State, StateContext } from '@ngxs/store';
import { EMPTY, Observable, catchError, concatMap, from, map, switchMap, tap, toArray } from 'rxjs';
import {
  DataImport,
  ImportColumn,
  ImportProblem,
  ImportProblemCode,
  ImportStatus,
  ImportSuggestion,
} from '@exa/services/local-api/local-api.models';
import { LocalApiService, problemDetail } from '@exa/services/local-api/local-api.service';
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

export type ImportPhase = 'choose' | 'uploading' | 'suggestion' | 'running' | 'done';

export interface UploadProgress {
  name: string;
  size: number;
  /** The fraction sent so far. */
  sent: number;
}

/** The table as the user edits it. */
export interface ImportDraft {
  schema: string;
  table: string;
  columns: ImportColumn[];
}

export interface ImportPage {
  phase: ImportPhase;
  uploads: UploadProgress[];
  uploadId?: string;
  /** Why the chosen files cannot be imported. */
  fileError?: string;
  suggestion?: ImportSuggestion;
  draft?: ImportDraft;
  current?: DataImport;
  /** Why the import failed or did not start. */
  importError?: string;
  starting: boolean;
  /** The deployment's imports since the server started, newest first. */
  recent: DataImport[];
}

export interface ImportsStateModel {
  pages: { [name: string]: ImportPage };
}

export const emptyImportPage = (): ImportPage => ({ phase: 'choose', uploads: [], starting: false, recent: [] });

export const draftOf = (suggestion: ImportSuggestion): ImportDraft => ({
  schema: suggestion.schema,
  table: suggestion.table,
  columns: suggestion.columns.map(column => ({ name: column.name, type: column.type ?? '' })),
});

/** The suggestion's problems that the draft has not solved. */
export const openProblems = (page: ImportPage): ImportProblem[] => {
  const { suggestion, draft } = page;
  if (!suggestion || !draft) return [];
  return suggestion.problems.filter(
    problem =>
      problem.code !== ImportProblemCode.TABLE_TAKEN ||
      (draft.schema === suggestion.schema && draft.table === suggestion.table)
  );
};

export const canImport = (page: ImportPage): boolean => {
  const draft = page.draft;
  return (
    page.phase === 'suggestion' &&
    !page.starting &&
    !!draft &&
    openProblems(page).length === 0 &&
    draft.schema.trim() !== '' &&
    draft.table.trim() !== '' &&
    draft.columns.every(column => column.name.trim() !== '' && column.type.trim() !== '')
  );
};

const unchanged = (draft: ImportDraft, suggestion: ImportSuggestion): boolean =>
  draft.columns.every(
    (column, index) =>
      column.name === suggestion.columns[index]?.name && column.type === suggestion.columns[index]?.type
  );

const replaceImport = (recent: DataImport[], found: DataImport): DataImport[] =>
  recent.some(entry => entry.id === found.id)
    ? recent.map(entry => (entry.id === found.id ? found : entry))
    : [found, ...recent];

@State<ImportsStateModel>({
  name: 'imports',
  defaults: { pages: {} },
})
@Injectable()
export class ImportsState {
  private readonly api = inject(LocalApiService);

  @Selector()
  static pages(state: ImportsStateModel): ImportsStateModel['pages'] {
    return state.pages;
  }

  @Action(LoadImportsAction)
  load(ctx: StateContext<ImportsStateModel>, { name }: LoadImportsAction): Observable<unknown> {
    return this.api.listImports(name).pipe(
      tap(recent =>
        this.update(ctx, name, page => {
          const running = recent.find(entry => entry.status === ImportStatus.RUNNING);
          if (page.phase === 'choose' && !page.current && running) {
            return { ...page, recent, phase: 'running', current: running };
          }
          return { ...page, recent };
        })
      ),
      catchError(() => EMPTY)
    );
  }

  @Action(UploadFilesAction)
  upload(ctx: StateContext<ImportsStateModel>, { name, files }: UploadFilesAction): Observable<unknown> {
    this.update(ctx, name, page => ({
      ...emptyImportPage(),
      recent: page.recent,
      phase: 'uploading',
      uploads: files.map(file => ({ name: file.name, size: file.size, sent: 0 })),
    }));
    const sent = (index: number, fraction: number) =>
      this.update(ctx, name, page => ({
        ...page,
        uploads: page.uploads.map((upload, at) => (at === index ? { ...upload, sent: fraction } : upload)),
      }));

    return this.api
      .createUpload(
        name,
        files.map(file => ({ name: file.name, size: file.size }))
      )
      .pipe(
        switchMap(upload =>
          from(upload.files.map((file, index) => ({ file, index }))).pipe(
            concatMap(({ file, index }) =>
              this.api.uploadFile(file.url ?? '', files[index]).pipe(tap(fraction => sent(index, fraction)))
            ),
            toArray(),
            map(() => upload.id)
          )
        ),
        switchMap(uploadId =>
          this.api
            .suggestImport(name, { kind: 'upload', upload_id: uploadId })
            .pipe(map(suggestion => ({ uploadId, suggestion })))
        ),
        tap(({ uploadId, suggestion }) =>
          this.update(ctx, name, page => ({
            ...page,
            phase: 'suggestion',
            uploadId,
            suggestion,
            draft: draftOf(suggestion),
          }))
        ),
        catchError(error => {
          this.update(ctx, name, page => ({ ...page, phase: 'choose', uploads: [], fileError: problemDetail(error) }));
          return EMPTY;
        })
      );
  }

  @Action(EditImportTableAction)
  editTable(ctx: StateContext<ImportsStateModel>, { name, table }: EditImportTableAction): void {
    this.update(ctx, name, page => (page.draft ? { ...page, draft: { ...page.draft, ...table } } : page));
  }

  @Action(EditImportColumnAction)
  editColumn(ctx: StateContext<ImportsStateModel>, { name, index, column }: EditImportColumnAction): void {
    this.update(ctx, name, page =>
      page.draft
        ? {
            ...page,
            draft: {
              ...page.draft,
              columns: page.draft.columns.map((existing, at) => (at === index ? { ...existing, ...column } : existing)),
            },
          }
        : page
    );
  }

  @Action(ResetImportAction)
  reset(ctx: StateContext<ImportsStateModel>, { name }: ResetImportAction): void {
    this.update(ctx, name, page => (page.suggestion ? { ...page, draft: draftOf(page.suggestion) } : page));
  }

  @Action(StartImportAction)
  start(ctx: StateContext<ImportsStateModel>, { name }: StartImportAction): Observable<unknown> {
    const page = this.page(ctx, name);
    const { draft, suggestion, uploadId } = page;
    if (!canImport(page) || !draft || !suggestion || !uploadId) return EMPTY;

    this.update(ctx, name, current => ({ ...current, starting: true, importError: undefined }));
    return this.api
      .startImport(name, {
        source: { kind: 'upload', upload_id: uploadId },
        schema: draft.schema.trim(),
        table: draft.table.trim(),
        columns: unchanged(draft, suggestion) ? undefined : draft.columns,
      })
      .pipe(
        tap(started =>
          this.update(ctx, name, current => ({
            ...current,
            phase: 'running',
            starting: false,
            current: started,
            recent: replaceImport(current.recent, started),
          }))
        ),
        catchError(error => {
          this.update(ctx, name, current => ({ ...current, starting: false, importError: problemDetail(error) }));
          return EMPTY;
        })
      );
  }

  @Action(RefreshImportAction)
  refresh(ctx: StateContext<ImportsStateModel>, { name }: RefreshImportAction): Observable<unknown> {
    const running = this.page(ctx, name).current;
    if (!running || running.status !== ImportStatus.RUNNING) return EMPTY;

    return this.api.getImport(name, running.id).pipe(
      tap(found =>
        this.update(ctx, name, page => {
          const updated = { ...page, current: found, recent: replaceImport(page.recent, found) };
          switch (found.status) {
            case ImportStatus.SUCCEEDED:
              return { ...updated, phase: 'done' };
            case ImportStatus.FAILED:
            case ImportStatus.CANCELLED:
              return {
                ...updated,
                phase: page.suggestion ? 'suggestion' : 'choose',
                importError: found.status === ImportStatus.FAILED ? found.message : undefined,
              };
            default:
              return updated;
          }
        })
      ),
      catchError(() => EMPTY)
    );
  }

  @Action(CancelImportAction)
  cancel(ctx: StateContext<ImportsStateModel>, { name }: CancelImportAction): Observable<unknown> {
    const running = this.page(ctx, name).current;
    if (!running || running.status !== ImportStatus.RUNNING) return EMPTY;

    return this.api.cancelImport(name, running.id).pipe(
      switchMap(() => ctx.dispatch(new RefreshImportAction(name))),
      catchError(() => EMPTY)
    );
  }

  @Action(ChooseFilesAgainAction)
  chooseAgain(ctx: StateContext<ImportsStateModel>, { name }: ChooseFilesAgainAction): void {
    this.update(ctx, name, page => ({ ...emptyImportPage(), recent: page.recent }));
  }

  private page(ctx: StateContext<ImportsStateModel>, name: string): ImportPage {
    return ctx.getState().pages[name] ?? emptyImportPage();
  }

  private update(ctx: StateContext<ImportsStateModel>, name: string, change: (page: ImportPage) => ImportPage): void {
    const pages = ctx.getState().pages;
    ctx.setState({ pages: { ...pages, [name]: change(pages[name] ?? emptyImportPage()) } });
  }
}
