# Contributing to LiveTemplate Examples

Thank you for your interest in contributing examples!

## Adding a New Example

### 1. Create Example Directory

```bash
mkdir my-example
cd my-example
```

### 2. Create go.mod

```go
module my-example

go 1.25

require github.com/livetemplate/livetemplate v0.1.0

// If using E2E tests:
// require github.com/livetemplate/lvt v0.1.0
```

### 3. Write Example Code

```go
package main

import (
    "log"
    "net/http"

    lt "github.com/livetemplate/livetemplate"
)

func main() {
    // Your example code
}
```

### 4. Add README.md

Create `my-example/README.md` explaining:
- What the example demonstrates
- How to run it
- Key features
- Any prerequisites

### 5. Add E2E Tests (Recommended)

Use Chromedp for browser-based E2E tests:

```go
package main

import (
    "testing"

    lvttest "github.com/livetemplate/lvt/testing"
)

func TestMyExample(t *testing.T) {
    // Your E2E test
}
```

### 6. Update Main README

Add your example to the main `README.md` with:
- Example name and description
- Directory path
- How to run
- Key features

## Example Guidelines

### Code Quality

- Follow Go best practices
- Add comments for complex logic
- Handle errors properly
- Use meaningful variable names

### Documentation

- Clear README in example directory
- Code comments explaining key concepts
- Step-by-step setup instructions

### Testing

- Add E2E tests using Chromedp
- Test happy paths and edge cases
- Verify UI updates correctly
- Test WebSocket communication

### Client Library

- Use CDN version for production examples
- Reference client library version in comments
- Show both CDN and local dev setup

### Dependencies

- Minimize external dependencies
- Use standard library when possible
- Document any required dependencies

## Local Development with Core Library

If you need to test your example against unreleased core library or LVT changes, you have two options:

### Recommended: Go Workspace (Automatic)

The **easiest way** - Go automatically uses local modules without any `go.mod` changes:

```bash
# From parent directory containing all repos
cd ..
./setup-workspace.sh

# Now test examples with local core library and LVT
cd examples
./test-all.sh  # Uses local livetemplate + lvt

# Or test individual example
cd counter
go test -v  # Automatically uses ../livetemplate and ../lvt
```

The workspace setup is done once and affects all repositories. See the [core library CONTRIBUTING.md](https://github.com/livetemplate/livetemplate/blob/main/CONTRIBUTING.md#testing-core-changes-with-lvtexamples) for details.

### Alternative: Manual Replace Directives

If you prefer manual control:

```bash
# Enable local development mode for all examples
./scripts/setup-local-dev.sh

# Test with local libraries
./test-all.sh

# Revert to published versions
./scripts/setup-local-dev.sh --undo
```

**Directory structure for both methods:**
```
parent/
├── livetemplate/  (core library)
├── lvt/           (CLI tool)
└── examples/      (this repo)
```

## Testing Your Example

```bash
# Run the example
cd my-example
go run main.go

# Run E2E tests
go test -v

# Test all examples together
cd ..
./test-all.sh
```

## Submitting Your Example

1. Fork the repository
2. Create a branch: `git checkout -b example/my-example`
3. Add your example
4. Update main README.md
5. Test thoroughly
6. Commit: `git commit -m "Add my-example demonstrating X"`
7. Push: `git push origin example/my-example`
8. Create Pull Request

## Example Categories

Consider these categories for new examples:

- **Basic**: Simple concepts (counter, hello world)
- **CRUD**: Database operations
- **Real-time**: WebSocket, chat, collaboration
- **Forms**: Validation, file uploads
- **Authentication**: Login, sessions, JWT
- **Testing**: E2E patterns, test helpers
- **Production**: Deployment, monitoring, scaling
- **Patterns**: Common UI patterns, best practices

## Code Style

- Use `gofmt` for formatting
- Follow [Effective Go](https://go.dev/doc/effective_go)
- Keep functions focused and small
- Add godoc comments for exported functions

## Questions?

- **Issues**: [GitHub Issues](https://github.com/livetemplate/examples/issues)
- **Discussions**: [GitHub Discussions](https://github.com/livetemplate/examples/discussions)

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
