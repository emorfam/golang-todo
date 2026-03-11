
  ┌──────────┬─────────────────────────────────────────────────────────────────────────────────────────────┐
  │ Severity │                                            Issue                                            │
  ├──────────┼─────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Minor    │ toolchain pinned to go1.26.0, not a 1.24.x patch as specified                               │
  ├──────────┼─────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Minor    │ trace_id is never injected into the request-scoped context logger (helper exists, not       │
  │          │ called)                                                                                     │
  ├──────────┼─────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Minor    │ pgxpool not used; pgx/v5/stdlib + database/sql used instead (accepted deviation)            │
  ├──────────┼─────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Minor    │ UUID column type in Postgres is TEXT, not the native UUID type as specified                 │
  ├──────────┼─────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Design   │ repository/todo.go imports metrics/ — violates the "repository imports domain only" rule    │
  ├──────────┼─────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Design   │ service/todo.go imports internal/apierror — violates the "service imports domain +          │
  │          │ repository interfaces only" rule (accepted as pragmatic)                                    │
  ├──────────┼─────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Design   │ No authorization logic in the service layer — the spec states it lives there; currently     │
  │          │ only authentication (JWT) is enforced, in the handler                                       │
  ├──────────┼─────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Design   │ Service "owns transactions" — no db.BeginTx usage anywhere (low-risk for single-entity      │
  │          │ CRUD)                                                                                       │
  ├──────────┼─────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Design   │ Middleware chain includes MetricsMiddleware not in spec; Tracing appears as outer otelhttp  │
  │          │ wrap rather than a named chain entry                                                        │
  └──────────┴─────────────────────────────────────────────────────────────────────────────────────────────┘


