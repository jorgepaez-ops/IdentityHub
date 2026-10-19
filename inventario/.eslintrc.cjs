// Lint rules for Inventario. The two restrictions mirror the security baseline (RNF-009, AM-015).
const forbiddenEval = {
  object: 'window',
  property: 'eval',
  message: 'eval() is forbidden (RNF-009).',
}

const forbiddenHtmlInjection = {
  selector: 'JSXAttribute[name.name="dangerouslySetInnerHTML"]',
  message: 'dangerouslySetInnerHTML opens the door to XSS (AM-015); record an ADR before using it.',
}

module.exports = {
  root: true,
  ignorePatterns: ['dist', 'node_modules'],
  env: { browser: true, es2022: true },
  parser: '@typescript-eslint/parser',
  parserOptions: { ecmaVersion: 'latest', sourceType: 'module' },
  plugins: ['@typescript-eslint', 'react-hooks'],
  extends: ['eslint:recommended', 'plugin:@typescript-eslint/recommended'],
  rules: {
    'react-hooks/rules-of-hooks': 'error',
    'react-hooks/exhaustive-deps': 'warn',
    'no-restricted-properties': ['error', forbiddenEval],
    'no-restricted-syntax': ['error', forbiddenHtmlInjection],
  },
}
