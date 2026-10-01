module.exports = {
  root: true,
  env: { browser: true, es2022: true },
  parser: '@typescript-eslint/parser',
  parserOptions: { ecmaVersion: 'latest', sourceType: 'module' },
  plugins: ['@typescript-eslint', 'react-hooks'],
  extends: [
    'eslint:recommended',
    'plugin:@typescript-eslint/recommended',
  ],
  ignorePatterns: ['dist', 'node_modules'],
  rules: {
    'react-hooks/rules-of-hooks': 'error',
    'react-hooks/exhaustive-deps': 'warn',

    'no-restricted-properties': [
      'error',
      {
        object: 'window',
        property: 'eval',
        message: 'eval() está prohibido: ver RNF-009.',
      },
    ],
    // RNF-009 / AM-015: este selector JSX prohíbe la prop que inyecta HTML.
    'no-restricted-syntax': [
      'error',
      {
        selector: 'JSXAttribute[name.name="dangerouslySetInnerHTML"]',
        message:
          'dangerouslySetInnerHTML abre la puerta a XSS (AM-015). Si de verdad hace falta, documentarlo en un ADR.',
      },
    ],
  },
}
