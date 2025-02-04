# Agent Go

A Go implementation of an AI agent orchestration system

## Project Structure

```
agent-go/
├── cmd/
│   └── main.go           # Application entry point
├── pkg/
│   ├── agents/           # Agent-related code
│   ├── tasks/            # Task definitions and handling
│   ├── tools/            # Tool interfaces and implementations
│   ├── team/             # Team orchestration (formerly team)
│   └── memory/           # Memory management
└── internal/
    └── utils/            # Internal utilities
```

## Getting Started

1. Clone the repository
2. Install Go 1.21 or later
3. Run the application:
   ```bash
   go run cmd/main.go
   ```

## Features

- Agent-based task execution
- Team orchestration
- Tool integration
- Memory management

## License

MIT License 