module.exports = {
  preset: 'jest-preset-angular',
  setupFilesAfterEnv: ['<rootDir>/jest-setup.ts'],
  testEnvironment: 'jsdom',
  testMatch: ['<rootDir>/src/**/*.spec.ts'],
  moduleNameMapper: {
    '^@exa/(.*)$': '<rootDir>/src/app/$1',
    // The package's main entry is an ES5 bundle, which cannot extend the ES2015 classes of its core.
    '^@ag-grid-community/angular$': '<rootDir>/node_modules/@ag-grid-community/angular/fesm2015/ag-grid-community-angular.js',
  },
  transform: {
    '^.+\\.(ts|js|html)$': [
      'jest-preset-angular',
      {
        tsconfig: '<rootDir>/tsconfig.spec.json',
        stringifyContentPathRegex: '\\.(html|svg)$',
      },
    ],
  },
  moduleFileExtensions: ['ts', 'html', 'js', 'json', 'mjs'],
  transformIgnorePatterns: ['node_modules/(?!(.*\\.mjs$|@ag-grid-community/angular/))'],
};
