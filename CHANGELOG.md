# Changelog

All notable changes to LiveTemplate Examples will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v0.1.0] - 2025-11-03

Initial release of LiveTemplate Examples as a standalone repository.

### Examples Included

1. **Counter** - Simple state management with reactive updates
2. **Chat** - Multi-user chat application with WebSocket
3. **Todos** - Full CRUD application with SQLite database
4. **Graceful Shutdown** - Proper server shutdown handling
5. **Observability** - Logging, metrics, and tracing
6. **Testing** - E2E testing patterns with Chromedp
7. **Production** - Production deployment configuration
8. **Trace Correlation** - Request tracing and correlation IDs

### Features

- **Self-contained Examples**: Each example has its own go.mod
- **E2E Testing**: Chromedp-based browser tests
- **CDN Integration**: Examples use CDN version of client library
- **Documentation**: README for each example with setup instructions
- **Production Patterns**: Real-world deployment examples

### Infrastructure

- Go module configuration for all examples
- Import paths updated for extracted repositories
- .gitignore for build artifacts
- VERSION file for release tracking

### Documentation

- Complete main README with all examples
- Contributing guidelines for new examples
- Individual README files per example

### Related Versions

- Core Library: v0.1.0
- Client Library: v0.1.0
- LVT CLI: v0.1.0

---

## Version Synchronization

Examples follow the LiveTemplate core library's major.minor version (X.Y):

- Patch versions (X.Y.Z) are independent
- Minor/major versions must match core library
- See README.md for details
