# Distributed & Parallel Game of Life Simulator

> A concurrent and distributed implementation of Conway's Game of Life, developed using Go as part of an academic project at the **University of Bristol**. This project expands upon base code provided in coursework, with significant additions for performance evaluation, modularization, and visual output.

---

## Overview

This project explores how complex emergent behavior arises from simple rules using concurrency and distributed computing. It includes two independent versions:

- **Parallel Version**: High-performance multithreaded execution using Goroutines.
- **Distributed Version**: Node-based execution across a simulated or cloud-based network.

Both versions support modular development, testing, benchmarking, and visualization.

### Key Features
- **Modular Architecture**: Organized into `gol`, `check`, `sdl`, `util`, plus version-specific folders (`parallel/`, `distributed/`)
- **High Performance**: Parallelism with Goroutines
- **Distributed Execution**: Includes RPC communication and broker coordination
- **Robust Testing**: Unit tests and benchmarks for each version
- **Data Analysis & Visualization**: CSV output + `plot.py` for charting

---

## Project Structure

```bash
.
├── distributed/                # Distributed version
│   ├── awsNode/                # Cloud node execution logic
│   ├── broker/                 # Distributed node coordination
│   ├── check/ / gol/ / util/   # Shared logic modules
│   ├── images/ / out/ / sdl/   # Outputs & visual
│   ├── *.go                    # Main + test files
│   ├── go.mod / go.sum         # Go modules
│   ├── plot.py / results.csv   # Performance charting
│   └── LICENSE / README.md
├── parallel/                   # Parallel version
│   ├── check/ / gol/ / util/   # Local computation modules
│   ├── images/ / out/ / sdl/   # Outputs & visual
│   ├── *.go                    # Main + test files
│   ├── go.mod / go.sum         # Go modules
│   ├── plot.py / results.csv   # Performance charting
│   └── LICENSE / README.md
```

---

## Requirements

- Go >= 1.17
- Python >= 3.6 (for plotting, requires `matplotlib`)
- SDL2 (for optional graphical mode)

---

## Testing Suite (Examples)

```bash
# Run core Game of Life logic tests
go test -v gol/gol_test.go

# Run benchmark performance tests
go test -v -bench .

# Test keyboard or rendering interactions (SDL mode)
go test -v sdl/sdl_test.go

# Run trace or utility test modules
go test trace/trace_test.go
```

Each version contains independent test files following the same structure.

---

## Visualization & Analysis

- `results.csv`: Records metrics such as simulation time per turn
- `plot.py`: Generates line charts using matplotlib

---

## Tech Stack

| Category            | Technology             | Description                                |
|---------------------|-------------------------|--------------------------------------------|
| Programming         | Go                     | Concurrency, RPC communication             |
| Visualization       | SDL2                   | Real-time GUI (optional)                   |
| Testing             | Go testing framework   | Unit & benchmark tests                     |
| Data Visualization  | Python + Matplotlib    | CSV plotting and metrics analysis          |
| Distributed System  | Goroutines + custom RPC| Node coordination and state synchronization|
| Dependency Manager  | Go Modules             | Module management and version control      |

---

## License

This project is licensed under the [Apache License 2.0](./LICENSE).

> **Notice**: This project contains partial code originally provided by a course assignment at the **University of Bristol**. It has been extended and modified by the student for educational and portfolio purposes. All base intellectual property belongs to the university and original authors.

