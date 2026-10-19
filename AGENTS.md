# Repository Guidelines

## Project Structure & Module Organization

Online Boutique is a polyglot microservices application. `compose.yaml` defines the local stack and exposes the frontend at `http://localhost:8080`. Service source lives in `src/<service>/`; notable nested projects include `src/cartservice/src/` and `src/cartservice/tests/`. Shared gRPC contracts are in `protos/`, while generated client code is kept with each service. Documentation and architecture images live in `docs/`. Keep service-specific Dockerfiles, dependency manifests, and generated artifacts inside their owning service directory.

## Build, Test, and Development Commands

Run these commands from the repository root unless noted otherwise:

- `docker compose up --build -d` builds and starts the complete local application.
- `docker compose ps` checks container status; `docker compose logs -f <service>` follows a service's logs.
- `docker compose down` stops and removes the stack.
- `docker compose build frontend` rebuilds one service after a focused change.
- `go test ./...` runs tests from an individual Go service directory, for example `src/frontend/`.
- `dotnet test src/cartservice/tests/cartservice.tests.csproj` runs the xUnit tests for cartservice.
- `cd src/adservice && ./gradlew test` runs Java/Gradle tests when modifying adservice.

Node.js services currently have no implemented test script; do not treat `npm test` as a passing check.

## Coding Style & Naming Conventions

Follow the conventions of the service you change. Format Go with `gofmt` (tabs; `*_test.go` test files). Use four spaces for C# and Python, and two spaces for JavaScript. Java uses the configured `google-java-format` Gradle plugin; keep Java changes compatible with it. Use lowercase service directory names such as `productcatalogservice`, descriptive Go test names such as `TestParseCatalog`, and clear environment-variable names such as `PRODUCT_CATALOG_SERVICE_ADDR`.

## Testing Guidelines

Add or update focused tests alongside behavior changes: Go tests sit next to the package, while cartservice tests belong in `src/cartservice/tests/`. Exercise the affected Compose service after changes that alter networking, environment variables, or Docker builds. No repository-wide coverage threshold is configured; prioritize regression tests for bug fixes and integration-sensitive paths.

## Commit & Pull Request Guidelines

Recent history uses short imperative subjects, often with Conventional Commit prefixes: `refactor: revise project setup` or `chore(deps): update dotnet`. Keep commits focused and avoid mixing generated-code updates with unrelated cleanup. Pull requests should explain the affected services, list validation commands, link any issue, and include screenshots for visible frontend changes. Call out changes to `compose.yaml`, ports, environment variables, or Protocol Buffers explicitly.
