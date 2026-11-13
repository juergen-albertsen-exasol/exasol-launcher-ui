import { ComponentFixture, TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { By } from '@angular/platform-browser';
import { DangerDialogComponent, DangerDialogData } from './danger-dialog.component';

describe('DangerDialogComponent', () => {
  let fixture: ComponentFixture<DangerDialogComponent>;
  let close: jest.Mock;

  const render = async (data: DangerDialogData) => {
    close = jest.fn();
    await TestBed.configureTestingModule({
      imports: [DangerDialogComponent],
      providers: [
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close } },
      ],
    }).compileComponents();
    fixture = TestBed.createComponent(DangerDialogComponent);
    fixture.detectChanges();
  };

  const element = <T extends HTMLElement>(selector: string) =>
    fixture.debugElement.query(By.css(selector)).nativeElement as T;
  const type = (value: string) => {
    const input = element<HTMLInputElement>('.danger-dialog__input');
    input.value = value;
    input.dispatchEvent(new Event('input'));
    fixture.detectChanges();
  };
  const confirmButton = () => element<HTMLButtonElement>('.danger-dialog__confirm');

  it('enables Destroy only once the deployment name is typed', async () => {
    await render({ deployment: 'default', path: '/deployments/default', action: 'destroy' });

    expect(element('.danger-dialog__title').textContent?.trim()).toBe('Destroy default?');
    type('defaul');
    expect(confirmButton().disabled).toBe(true);

    type('default');
    expect(confirmButton().disabled).toBe(false);
    confirmButton().click();
    expect(close).toHaveBeenCalledWith('destroy');
  });

  it('destroys and removes the folder when the checkbox is checked', async () => {
    await render({ deployment: 'default', path: '/deployments/default', action: 'destroy' });

    expect(element('.danger-dialog__check').textContent).toContain('/deployments/default');
    element<HTMLInputElement>('.danger-dialog__remove').click();
    type('default');
    confirmButton().click();

    expect(close).toHaveBeenCalledWith('destroy_and_remove');
  });

  it('opens Destroy and remove with the checkbox checked', async () => {
    await render({ deployment: 'default', path: '/deployments/default', action: 'destroy', removeFolder: true });

    expect(element<HTMLInputElement>('.danger-dialog__remove').checked).toBe(true);
    type('default');
    confirmButton().click();

    expect(close).toHaveBeenCalledWith('destroy_and_remove');
  });

  it('removes only the folder and names it', async () => {
    await render({ deployment: 'test', path: '/deployments/test', action: 'remove' });

    expect(element('.danger-dialog__title').textContent?.trim()).toBe('Remove the folder of test?');
    expect(element('.danger-dialog__body').textContent).toContain('Folder to remove: /deployments/test');
    expect(fixture.debugElement.query(By.css('.danger-dialog__remove'))).toBeNull();
    type('test');
    confirmButton().click();

    expect(close).toHaveBeenCalledWith('remove');
  });

  it('closes without an operation on Cancel', async () => {
    await render({ deployment: 'test', path: '/deployments/test', action: 'remove' });

    element<HTMLButtonElement>('.danger-dialog__cancel').click();

    expect(close).toHaveBeenCalledWith();
  });
});
