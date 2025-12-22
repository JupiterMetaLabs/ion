# Contributing to Ion

Thank you for your interest in contributing to Ion! We welcome contributions from the community to make this the best logging library for JupiterMeta services.

## Development Setup

1.  **Clone the repository**:
    ```bash
    git clone https://github.com/JupiterMetaLabs/ion
    cd ion
    ```

2.  **Install dependencies**:
    ```bash
    go mod download
    ```

## Project Structure

You might notice that core files like `zap.go` and `config.go` are in the root directory. This is intentional:

1.  **Import Cleanliness**: It allows users to import `github.com/JupiterMetaLabs/ion` directly, without a redundant suffix like `/pkg/ion`.
2.  **Circular Dependencies**: Moving implementation files to sub-packages often creates cycles with the `Config` struct, which must remain in the root API.
3.  **Internal Hiding**: Complex implementation details (like OpenTelemetry wiring) are hidden in `internal/otel` to keep the public API clean.

## Coding Standards

-   **Go Version**: We target Go 1.24+.
-   **Formatting**: code must be formatted with `gofmt` (or `goimports`).
-   **Linting**: We use strict linting rules. Ensure your code passes `golangci-lint`:
    ```bash
    make lint
    ```
-   **Testing**: New features must include unit tests. Maintain high code coverage.
    ```bash
    make test
    ```
-   **No Panics**: Avoid panics in library code. Return errors instead.

## Pull Request Process

1.  Create a feature branch from `main`.
2.  Implement your changes with tests.
3.  Ensure CI passes locally (`make test lint`).
4.  Submit a Pull Request with a clear description of the changes.

## Release Process

Semantic versioning is used. Releases are tagged automatically via CI or manually by maintainers.
