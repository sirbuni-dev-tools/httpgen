# httpgen

Generate IntelliJ HTTP Client request files and JSON payloads from Spring Boot source code, from the terminal. It writes the same `.http` files as the API HTTP Generator plugin for IntelliJ IDEA, for terminal users, VS Code users and LLM agents.

> Early development: the help system is in place; `--req`, `--json`, `--base64` and `--list` arrive in the next releases.

## Usage

```bash
httpgen --help
```

```bash
httpgen --help req
```

Every option has its own page (`httpgen --help <topic>`, or `httpgen --req --help`), and `httpgen --man` prints the whole manual as a man page:

```bash
httpgen --man > /usr/local/share/man/man1/httpgen.1
```

## Build

Needs Go 1.26.

```bash
make test build
```

The binary lands in `bin/httpgen`.
