# ClouseStone - Project Instructions

## Go Development

Before writing or modifying Go code, consult `.agents/skills/golang-pro/SKILL.md` and load the relevant reference files based on the task:

- Concurrency (goroutines, channels, sync): `references/concurrency.md`
- Interfaces and composition: `references/interfaces.md`
- Generics: `references/generics.md`
- Testing and benchmarks: `references/testing.md`
- Project structure and modules: `references/project-structure.md`

## Project Overview

- **Language**: Go
- **Database**: MongoDB
- **Architecture**: REST API with JWT authentication
- **Structure**: `internal/mainservice/` contains handlers, models, modules, database, and utilities

## API Documentation (MANDATORY)

There is a shared context folder for **another AI model** that builds clients
(native app, PWA, integrations) without reading this backend's source:
`D:\Gluzie\Documentacion Gluzie\`.

**Whenever you add, modify, or remove an endpoint or a JSON model, you MUST update
this documentation in the same change.** Follow the process in
`D:\Gluzie\Documentacion Gluzie\_mantenimiento\CONVENCIONES.md`:

- Entry point is `README.md`; docs are fragmented (one file per endpoint under
  `endpoints/`) so the other model loads only what it needs.
- New endpoint → create `endpoints/<area>-<name>.md` + add to `endpoints/_INDICE.md`
  and the `README.md` table.
- Changed endpoint/model → update its endpoint file and/or `modelos/esquemas.md`.
- Auth/cookie/token changes → `01-autenticacion.md`; sync logic → `02-modelo-sincronizacion.md`;
  CORS/env/limits → `00-arquitectura.md`.
- Document what the code actually does, including quirks (mark with ⚠️). Never invent fields.
