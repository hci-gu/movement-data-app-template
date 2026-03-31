# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

CLI - Hjärtinfarkt is a Flutter iOS research app for Gothenburg University that tracks physical activity patterns (steps, walking metrics) before and after heart attacks using iOS HealthKit data. The entire UI is in Swedish.

## Build & Dev Commands

```bash
# Flutter app
flutter pub get                    # Install dependencies
flutter run                        # Run on connected iOS device/simulator
flutter build ios                  # Build iOS release
flutter analyze                    # Run lint analysis
flutter test                       # Run tests

# Local health package (must be fetched separately if modified)
cd health && flutter pub get

# Go backend (PocketBase)
cd api && go run main.go serve     # Run backend locally
cd api && go build -o app           # Build backend binary
```

## Architecture

### Two-Part System

**Flutter App (`lib/`)** — iOS-only Cupertino-styled mobile app
**Go Backend (`api/`)** — PocketBase with custom REST endpoints in `main.go`

### Flutter State Management

Uses **Hooks Riverpod** (`hooks_riverpod` + `flutter_hooks`). Key providers in:
- `lib/state/auth.dart` — `authProvider` (StateNotifier), `dataUploadedProvider` (StateProvider)
- `lib/screens/result/state.dart` — `eventDateProvider`, `displayModeProvider`, `stepDataProvider`, `chartDataProvider`, average step providers

### Routing

GoRouter in `lib/router.dart` with auth-based redirects:
- `/introduction` → Welcome/info (unauthenticated)
- `/introduction/login` → Swedish personal number login
- `/steps` → HealthKit permission + data upload (authenticated, pre-upload)
- `/` → Results dashboard with charts (authenticated, post-upload)

`RouterNotifier` listens to `authProvider` token changes and redirects based on auth state and `dataUploadedProvider`.

### Singletons

`Storage()`, `HealthManager()`, `Api()`, and the PocketBase client (`lib/pocketbase.dart`) are all singletons.

### Data Flow

1. User logs in with Swedish personal number (validated via `personnummer` package)
2. Accepts research consent → `POST /users` creates/finds user in PocketBase
3. App requests HealthKit access, fetches health data from 2021-01-01 onward
4. `Api.uploadDataInChunks()` splits data into 10 chunks, gzip-compresses each, sends to `POST /data`
5. Backend stores compressed JSON files per user at `/pb/pb_data/raw/{personalId}/`
6. Results screen fetches back via `GET /data/:personalId`, displays before/after charts using `fl_chart`

### Backend API Endpoints (`api/main.go`)

| Method | Path | Purpose |
|--------|------|---------|
| POST | `/users` | Find or create user + save consent |
| POST | `/data` | Upload gzip-compressed health data chunk |
| GET | `/data/:personalId` | Retrieve all uploaded health data |
| POST | `/info` | Save user info (no-data case) |
| POST | `/:id/form` | Save questionnaire answers |

### Local Health Package

`health/` is a local Flutter package (path dependency in pubspec.yaml) wrapping iOS HealthKit. It collects: steps, walking speed, steadiness, asymmetry percentage, double support percentage, and step length.

### Key Design Decisions

- Auth uses personal number as username with a random UUID password (security is not a primary concern — research context)
- Data is stored as gzip-compressed JSON files on disk, with metadata in PocketBase `dataUploads` collection
- Chart visualization normalizes event date to display bucket boundaries (day/week/month)
- Device with the most data points is automatically selected when multiple sources exist
