# Contributing to Hooklab

First off, thank you for considering contributing! 🎉

## Ways to Contribute

- 🐛 **Report bugs** — Open an issue with steps to reproduce
- 💡 **Suggest features** — Share ideas in discussions or issues
- 📖 **Improve docs** — Fix typos, clarify instructions
- 🔧 **Submit PRs** — Bug fixes, features, tests

---

## Getting Started

### Prerequisites
- Go 1.18 or later
- Git

### Setup
```sh
# Clone the repo
git clone https://github.com/essajiwa/hooklab.git
cd hooklab

# Run the server
go run .

# Run tests
go test -v

# Check coverage
go test -cover
```

---

## Good First Issues

New to the project? Look for issues labeled:

- [`good first issue`](https://github.com/essajiwa/hooklab/labels/good%20first%20issue) — Great for first-time contributors
- [`help wanted`](https://github.com/essajiwa/hooklab/labels/help%20wanted) — We'd love help with these
- [`documentation`](https://github.com/essajiwa/hooklab/labels/documentation) — Docs improvements

### Suggested First Contributions

| Task | Difficulty | Description |
|------|------------|-------------|
| Add `/health` endpoint | Easy | Return `{"status":"ok"}` for health checks |
| Add request count badge | Easy | Show total requests in UI header |
| Add dark/light mode toggle | Medium | Theme switcher in UI |
| Add request search/filter | Medium | Filter events by method, path, or body content |
| Add webhook signature validation | Medium | Verify signatures for Stripe, GitHub, etc. |
| Add request export (JSON/CSV) | Medium | Download event history |

---

## Pull Request Process

1. **Fork** the repo and create your branch from `master`
2. **Make changes** — Keep commits focused and atomic
3. **Add tests** — Maintain or improve coverage (currently 84%+)
4. **Update docs** — If you changed behavior, update README/docs
5. **Submit PR** — Fill out the PR template

### PR Checklist

- [ ] Code follows existing style
- [ ] Tests pass (`go test -v`)
- [ ] No new linter warnings (`go vet ./...`)
- [ ] Docs updated if needed

---

## Code Style

- Follow standard Go conventions (`gofmt`)
- Keep functions small and focused
- Add comments for exported functions
- Use meaningful variable names

### Project Structure

```
.
├── main.go          # Entry point, flags, graceful shutdown
├── app.go           # App state, events, subscribers
├── handlers.go      # HTTP handlers
├── sse.go           # Server-Sent Events logic
├── server.go        # Server setup, routing
├── main_test.go     # Tests
└── web/
    └── index.html   # Embedded React + Tailwind UI
```

---

## Reporting Bugs

When reporting bugs, please include:

1. **Go version** (`go version`)
2. **OS** (e.g., macOS 14, Ubuntu 22.04)
3. **Steps to reproduce**
4. **Expected vs actual behavior**
5. **Relevant logs or screenshots**

---

## Feature Requests

We welcome feature ideas! When suggesting:

1. **Describe the problem** you're trying to solve
2. **Propose a solution** (if you have one)
3. **Consider alternatives** you've thought about

---

## Code of Conduct

Be respectful and constructive. We're all here to learn and build something useful together.

---

## Questions?

Tag [@essajiwa](https://github.com/essajiwa) in an issue

---

Thank you for contributing! 🙏
