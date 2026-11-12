import '@testing-library/jest-dom';

// Initialize the Angular test env, but only when no setup file already did it.
//
// Under `ng test` (@angular-builders/jest), the builder's own setup-jest runs
// first and loads its own zone-testing bundle. Under `npx jest`, nothing runs
// first, so we initialize here.
//
// The require is *inside* the conditional on purpose: a top-level `import` of
// jest-preset-angular/setup-env/zone eagerly loads zone.js/testing, which can
// collide with the bundle the builder already loaded (different file paths
// land in the require cache as separate Zone instances and break ProxyZone).
//
// This check must NOT go through `@angular/core/testing` (e.g. `getTestBed()`):
// that module snapshots Zone's fakeAsync patch into a top-level const the
// moment it is first imported/required, so importing it before zone.js/testing
// is loaded permanently breaks `fakeAsync()` for the whole process, even if
// zone.js/testing is loaded right afterward. Checking the `Zone` global
// directly lets us decide *before* `@angular/core/testing` is ever touched.
const zoneGlobal = (global as any).Zone;
const zoneAlreadyPatched = zoneGlobal && zoneGlobal[zoneGlobal.__symbol__('fakeAsyncTest')];
if (!zoneAlreadyPatched) {
  // eslint-disable-next-line @typescript-eslint/no-require-imports, @typescript-eslint/no-var-requires
  const { setupZoneTestEnv } = require('jest-preset-angular/setup-env/zone');
  setupZoneTestEnv();
}
