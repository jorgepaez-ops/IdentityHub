# 0002 — Construir el IdP en lugar de integrar Keycloak

Estado: aceptada · 2026-09-05

## Contexto

Para una aplicación de gestión de identidad hay dos caminos: desplegar Keycloak
y que nuestro backend sea un mero *resource server*, o implementar el proveedor
de identidad nosotros.

## Decisión

Implementar el IdP en Go: emisión y rotación de tokens, hashing de contraseñas,
segundo factor, RBAC y auditoría.

## Justificación

Con Keycloak la aplicación propia se reduce a un CRUD delgado y el peso del
proyecto se desplaza a configurar un producto ajeno. Eso deja el pipeline sin
nada interesante que escanear: gosec y CodeQL no tendrían criptografía, manejo
de secretos ni control de acceso propio sobre los que informar. La decisión es
deliberadamente contraria a lo que se haría en producción, y por un motivo
académico explícito: **el objeto de estudio es el pipeline, y el pipeline
necesita código con riesgo real que analizar**.

## Consecuencias

- Asumimos escribir criptografía de autenticación, que es exactamente donde es
  fácil equivocarse. Se mitiga con el modelo de amenazas, pruebas dirigidas por
  amenaza (AM-003, AM-004) y los gates de SAST.
- No hay federación ni SSO. Documentado como fuera de alcance en `00-vision.md`.
- En un proyecto real la recomendación sería la contraria: no escribas tu propio
  IdP. Queda dicho aquí para que la decisión no se lea como ignorancia.
