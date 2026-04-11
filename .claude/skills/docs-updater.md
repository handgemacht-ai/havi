---
name: docs-updater
description: Instructions for keeping API documentation current when the API surface changes
---

# Docs Updater Skill

## When to Activate

Activate this skill when any of these change:

- REST endpoint added, removed, or modified (method, path, request/response shape)
- W3C annotation envelope structure changes (new body types, selector types, field rules)
- Postgres schema changes (new columns, tables, indexes, defaults)
- Error codes added or changed
- CORS configuration changes
- Query parameter filters added or modified

## What to Update

### API.md (primary contract)

- Endpoint table and per-endpoint sections
- W3C envelope examples (update all 3 if structure changes)
- Error codes table
- Postgres schema SQL (must match migration files exactly)
- CORS section

### .claude/skills/product/api-contract.md

- Endpoint summary table
- Data model description
- Design decisions (if a new decision is made)
- Boundaries (if scope changes)
- Extension points

### .claude/rules/api-conventions.md

- Resource table (if endpoints change)
- JSON envelope format (if response shape changes)
- Content types (if new content types are introduced)
- Filter query params (if new filters added)

### .claude/rules/w3c-annotations.md

- Envelope structure (if W3C fields change)
- Body types table
- Selector types table
- Motivation values

## How to Update

1. Make the code/schema change first
2. Update API.md to reflect the change
3. Verify all examples in API.md still parse correctly
4. Update the product skill if the change affects the API surface summary
5. Update rules files if conventions change
6. Ensure Postgres schema in API.md matches the latest migration file

## Verification

After updating, cross-check:
- API.md schema SQL matches `server/migrations/*.sql`
- API.md endpoint list matches `server/internal/` handler registrations
- W3C envelope examples conform to `.claude/rules/w3c-annotations.md`
- Error codes match handler implementations
- Product skill accurately summarizes the current state
