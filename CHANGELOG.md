# Changelog

All notable changes to this project will be documented in this file.
The format is based on [Keep a Changelog](https://keepachangelog.com/).

## [0.1.0] - 2026-04-11

### Added

#### Chrome Extension
- Chrome Manifest V3 extension foundation (manifest.json, background service worker)
- Keyboard shortcut activation (Ctrl+Shift+A / Cmd+Shift+A)
- Capture overlay with Cropper.js region selection and resize handles
- Fabric.js markup canvas with drawing tools (rectangle, arrow, highlight, text)
- Color picker with 6 preset colors
- Undo support (Ctrl+Z)
- Screenshot compositing (page screenshot + markup as composite PNG)
- Comment input panel with thumbnail preview
- W3C Web Annotation data packaging (FragmentSelector, SvgSelector, HttpRequestState)
- Success toast notification
- ESC to cancel at any stage
- Side panel with empty state and server URL configuration
- Vendored Fabric.js v6.4.3 and Cropper.js v1.6.2

#### Go Server
- Database layer with pgxpool connection management and automatic migration runner (ann-771.1)
- Domain model types: entity, W3C Web Annotation DTOs, DB record structs, typed error constants (ann-771.1)
- AnnotationRepo interface and PostgresRepo implementation with 7 query methods (ann-771.2)
- Service layer with W3C envelope construction, validation, denormalization, resolve-conflict (ann-771.3)
- REST controller with 7 endpoints: create, list, get, get-image, update, delete, resolve (ann-771.4)
- Health check endpoint at GET /health (ann-771.4)
- CORS middleware allowing chrome-extension://* and http://localhost:* origins (ann-771.4)
- Fire-and-forget webhook dispatcher (ann-771.4)
- Graceful shutdown on SIGINT/SIGTERM (ann-771.4)
- 22 scenarigo integration tests against real Postgres

### Dependencies
- github.com/jackc/pgx/v5
- github.com/google/uuid
- github.com/handgemacht-ai/scenarigo v0.6.0
