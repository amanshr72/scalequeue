import parser from '@typescript-eslint/parser';
export default [{
  files: ['src/**/*.ts'],
  languageOptions: { parser, ecmaVersion: 'latest', sourceType: 'module' },
  rules: {
    'constructor-super': 'error',
    'no-async-promise-executor': 'error',
    'no-constant-condition': 'error',
    'no-debugger': 'error',
    'no-dupe-args': 'error',
    'no-duplicate-case': 'error',
    'no-unreachable': 'error',
    'use-isnan': 'error',
    'valid-typeof': 'error',
  },
}];
