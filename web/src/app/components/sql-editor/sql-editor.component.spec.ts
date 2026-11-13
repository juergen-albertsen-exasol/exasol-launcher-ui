import { ComponentFixture, TestBed } from '@angular/core/testing';
import { SqlEditorComponent } from './sql-editor.component';

describe('SqlEditorComponent', () => {
  let fixture: ComponentFixture<SqlEditorComponent>;
  let editor: SqlEditorComponent;

  beforeEach(async () => {
    await TestBed.configureTestingModule({ imports: [SqlEditorComponent] }).compileComponents();
    fixture = TestBed.createComponent(SqlEditorComponent);
    fixture.componentRef.setInput('initialText', 'OPEN SCHEMA RETAIL;\n\nselect REGOIN_NAME from REGIONS;');
    fixture.detectChanges();
    editor = fixture.componentInstance;
  });

  const content = () => fixture.nativeElement.querySelector('.cm-content') as HTMLElement;
  const press = (init: KeyboardEventInit) =>
    content().dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true, ...init }));

  it('shows the initial text with line numbers and highlighted keywords', () => {
    expect(editor.text()).toBe('OPEN SCHEMA RETAIL;\n\nselect REGOIN_NAME from REGIONS;');
    expect(fixture.nativeElement.querySelectorAll('.cm-lineNumbers .cm-gutterElement').length).toBeGreaterThan(0);
    expect(content().getAttribute('aria-label')).toBe('SQL');
  });

  it('runs on Ctrl+Enter and runs all on Shift+Ctrl+Enter', () => {
    const run = jest.fn();
    const runAll = jest.fn();
    editor.run.subscribe(run);
    editor.runAll.subscribe(runAll);

    press({ ctrlKey: true });
    press({ ctrlKey: true, shiftKey: true });

    expect(run).toHaveBeenCalledTimes(1);
    expect(runAll).toHaveBeenCalledTimes(1);
  });

  it('reports the text, the selection, and the cursor', () => {
    const changes: string[] = [];
    editor.textChange.subscribe(text => changes.push(text));

    editor.select(21, 27);
    expect(editor.selection()).toEqual({ text: 'select', from: 21 });

    editor.insert('SELECT');
    expect(editor.text()).toBe('OPEN SCHEMA RETAIL;\n\nSELECT REGOIN_NAME from REGIONS;');
    expect(editor.cursor()).toBe(27);
    expect(editor.selection()).toBeUndefined();
    expect(changes).toEqual([editor.text()]);

    editor.replaceAll('SELECT 1');
    expect(editor.text()).toBe('SELECT 1');
  });

  it('underlines an error and marks its line until the next edit', () => {
    editor.markError(30);

    const mark = fixture.nativeElement.querySelector('.cm-sql-error') as HTMLElement;
    expect(mark.textContent).toBe('REGOIN_NAME');
    expect(fixture.nativeElement.querySelector('.cm-sql-error-line')?.textContent).toBe('3');

    editor.insert(' ');
    expect(fixture.nativeElement.querySelector('.cm-sql-error')).toBeNull();
  });

  it('underlines a qualified name as a whole', () => {
    editor.replaceAll('SELECT r.REGOIN_NAME, 1 FROM R');

    editor.markError(7);

    expect(fixture.nativeElement.querySelector('.cm-sql-error').textContent).toBe('r.REGOIN_NAME');
  });

  it('marks the lines of the running statement', () => {
    editor.markRunning({ from: 21, to: 52 });
    expect(fixture.nativeElement.querySelectorAll('.cm-sql-running')).toHaveLength(1);

    editor.markRunning(undefined);
    expect(fixture.nativeElement.querySelectorAll('.cm-sql-running')).toHaveLength(0);
  });

  it('accepts a completion with Tab and indents otherwise', async () => {
    fixture.componentRef.setInput('catalog', {
      schemas: [
        {
          name: 'RETAIL',
          virtual: false,
          columnsLoaded: true,
          tables: [{ name: 'SALES', kind: 'table', columns: [] }],
        },
      ],
    });
    fixture.detectChanges();
    editor.replaceAll('SELECT * FROM RETAIL.SA');
    editor.select(editor.text().length);

    content().dispatchEvent(new KeyboardEvent('keydown', { key: ' ', ctrlKey: true, bubbles: true, cancelable: true }));
    for (let attempt = 0; attempt < 20 && !fixture.nativeElement.querySelector('.cm-tooltip-autocomplete'); attempt++) {
      await new Promise(resolve => setTimeout(resolve, 20));
    }
    expect(fixture.nativeElement.querySelector('.cm-tooltip-autocomplete')).toBeTruthy();
    // CodeMirror ignores accepting a completion within its interaction delay after the list opens.
    await new Promise(resolve => setTimeout(resolve, 100));
    press({ key: 'Tab' });
    expect(editor.text()).toBe('SELECT * FROM RETAIL.SALES');

    editor.replaceAll('SELECT 1');
    editor.select(0);
    press({ key: 'Tab' });
    expect(editor.text()).toBe('  SELECT 1');
  });
});
