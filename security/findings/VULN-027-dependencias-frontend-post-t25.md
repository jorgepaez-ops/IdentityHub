# VULN-027 — Dependencias de build/frontend desactualizadas (vite, minimatch, react-router, undici)

| | |
|---|---|
| **Severidad** | CRITICAL y HIGH (moderate incluidas) |
| **Estado** | remediado |
| **Detectado por** | npm audit (`ci.yml`, job `5 · Dependencias vulnerables`) |
| **Componente** | `frontend/package.json`: `vite`/`esbuild` (vía `vitest`/`@vitest/coverage-v8`), `@typescript-eslint/parser` (vía `minimatch`), `react-router-dom`, `openapi-typescript` (vía `undici`) |
| **Avisos** | GHSA-67mh-4wv8-2f99 (esbuild); 3 ReDoS en `minimatch`; GHSA-wrjc-x8rr-h8h6 y GHSA-337j-9hxr-rhxg (react-router); 12 avisos de `undici` |
| **Amenaza** | AM-009 (dependencia comprometida en la cadena de suministro) |
| **Sembrada** | **NO** — hallazgo no previsto, igual que VULN-020 |
| **Evidencia antes** | `docs/evidencia/VULN-027/evidencia.json` |
| **Commit de remediación** | `d149e59` (T34a) |
| **Evidencia después** | `docs/evidencia/VULN-027/evidencia.json` (`npm audit`: 0 vulnerabilidades) |
| **Run de Actions (antes/después)** | https://github.com/jorgepaez-ops/IdentityHub/actions/runs/36259385478 / — (pendiente del próximo run de CI) |

## Cómo apareció

T25 (2026-09-26) retiró `axios` y `lodash` del proyecto porque no se usaban, y `npm audit` pasó de
17 a 15 hallazgos. Esos 15 ya estaban ahí desde antes — T25 los documentó como fuera de su alcance
(no relacionados con axios/lodash) — pero nunca tuvieron ficha propia ni id asignado. Al revisar el
PR de cierre de la Fase 3, se confirmó que ninguno tiene arreglo sin salto de versión mayor
(`npm audit fix` sin `--force` no resuelve nada).

## Evidencia

```
esbuild <=0.24.2 (moderate) — vía vite/vitest/@vitest/coverage-v8; fix: vite@8 (breaking)
minimatch 9.0.0-9.0.6, 3 ReDoS (high) — vía @typescript-eslint/parser; fix: @typescript-eslint/parser@8.70.1 (breaking)
react-router 6.0.0-7.17.0 (moderate) — open redirect + deserialización SSR; fix: react-router-dom@7.18.4 (breaking)
undici <=6.27.0, 12 avisos (high/critical) — vía openapi-typescript; fix: openapi-typescript@7.13.0 (breaking)

15 vulnerabilities (5 moderate, 8 high, 2 critical)
```

## Por qué importa en esta aplicación

De los cuatro, solo `react-router-dom` viaja al navegador (dependencia de producción de la SPA); el
resto son herramientas de build/lint/codegen que nunca llegan al bundle final. Aun así, un
`open redirect` en el router de una SPA de autenticación es un vector real de phishing (redirigir
tras el login a un dominio atacante), y dejar herramientas de build con CVE conocido sin razón
también es deuda técnica que un auditor señalaría.

## Remediación

Los cuatro subidos a la versión que corrige su aviso: `vite` 5→8, `vitest`/`@vitest/coverage-v8` 1→5,
`@vitejs/plugin-react` 4→6 (los peers nuevos de esta versión — `oxc-transform-react`,
`@rolldown/plugin-babel`, `babel-plugin-react-compiler` — son opcionales, no obligan a adoptar
Rolldown); `@typescript-eslint/parser`/`eslint-plugin` 6→8, con ESLint solo a 8.57 (no a 9/10: v8 de
typescript-eslint soporta `^8.57.0`, y `eslint-plugin-react-hooks@4.6.0` no soporta ESLint 9+, así
que subir ESLint no aportaba nada); `react-router-dom` 6→7 y `openapi-typescript` 6→7 (ninguno de los
dos se usa todavía en `src/`, riesgo cero). Único cambio de código: `vite.config.ts` importa
`defineConfig` de `"vitest/config"` en vez de `"vite"` (Vitest 5 dejó de fusionar la opción `test`
en el tipo `UserConfig` de Vite). `npm audit`: de 15 a 0. Verificado con el stack real: registro →
verificación → login por Nginx sigue devolviendo `200` con el `Set-Cookie` intacto y las cinco
cabeceras de RNF-009 presentes.
