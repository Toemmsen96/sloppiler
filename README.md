# Sloppiler
### *Beyond Deterministic Compilation*

> "We didn't reinvent the compiler. We asked an AI to guess what one looks like."
> — Sloppiler Engineering Blog, Issue 1 (Final)

**Sloppiler** is a next-generation, AI-first, LLM-native compilation platform that leverages the full generative power of large language models to holistically transform your source code into an executable binary artifact — at inference speed.

Traditional compilers are slow, opinionated, and constrained by decades of deterministic thinking. They parse. They typecheck. They enforce a single correct answer. Sloppiler moves compilation to the inference layer, reasoning about your *intent* rather than your *syntax* — eliminating the pedantic intermediate steps that have constrained the software development lifecycle for decades. The bottleneck is no longer your toolchain. It's your willingness to ship.

**Key differentiators:**
- Zero determinism — every build is a unique stakeholder experience
- Polyglot-native input layer — not bound by the grammar constraints of any single language specification
- Blazing-fast time-to-segfault
- Segfault-as-a-feature architecture with full core dump transparency
- Model-native code synthesis unconstrained by legacy compiler assumptions
- Agentic assembly pipeline (`--optimistic`) for synergistic human-AI co-compilation
- Fully on-premise inference — your data, your model, your binary
- Radical shift-left binary delivery: bypass intermediate toolchain layers and ship directly

**Sloppiler is the only compiler built on the insight that your code doesn't need to be *understood* — it needs to be *shipped*.**

## Roadmap

See the [Alphabet of Inevitability](ROADMAP.md).

## Contributing

See the [Contributor Excellence Framework](CONTRIBUTING.md).

## Requirements

- [Ollama](https://ollama.com) running locally with a model pulled
- Go 1.21+ to build
- `nasm` + `ld` (binutils) for `--optimistic` mode only

## Build

```sh
go build -o sloppiler .
```

Or use the helper script which handles everything:

```sh
./run.sh <source-file> [output]
MODEL=codellama ./run.sh main.c hello
```

## Usage

```sh
./sloppiler [options] <source-file>
```

| Flag | Default | Description |
|------|---------|-------------|
| `-model` | `llama3` | Ollama model to use |
| `-o` | `a.out` | Output binary path |
| `-ollama` | `http://localhost:11434/api/generate` | Ollama API URL |
| `--optimistic` | false | Engage agentic assembly co-pilot (requires nasm + ld) |
| `--loop N` | 0 | Re-align LLM outputs with ground truth up to N times on nasm failure |
| `--force-iterate N` | 0 | Proactively enhance output quality for N cycles even on success |

## Compilation Modalities

### Core Mode — *Frictionless Binary Ideation*

Instructs the model to emit the target binary as a hex-encoded byte sequence, which Sloppiler materializes directly to disk. An ELF header is synthesized to ensure kernel compatibility and binary executability. This is our flagship zero-abstraction, direct-to-segfault pipeline.

```sh
./sloppiler -model codellama main.c -o hello
./hello
# zsh: segmentation fault (core dumped) ./hello
```

### Optimistic Mode (`--optimistic`) — *Agentic Assembly Co-Pilot*

Engages the model as an assembly generation partner, routing output through `nasm` and `ld` for binary materialization. Assembly is generated holistically from source semantics, bypassing intermediate representation layers.

```sh
./sloppiler --optimistic -model codellama main.c -o hello
./hello
# zsh: segmentation fault (core dumped) ./hello
```

### Loop Mode (`--loop N`) — *Closed-Loop Error Remediation*

When assembly fails, the compiler errors and current output are fed back to the model for up to N self-correction cycles — a closed-loop remediation pipeline that continuously aligns output quality with ground truth.

```sh
./sloppiler --optimistic --loop 5 -model codellama main.c -o hello
```

### Force Iterate Mode (`--force-iterate N`) — *Continuous Improvement Pipeline*

Engages N improvement cycles even on successful compilation, feeding each output back to the model for further enhancement. A working binary is a starting point, not an endpoint.

```sh
./sloppiler --optimistic --force-iterate 3 -model codellama main.c -o hello
```

## Model recommendations

| Model | Behaviour |
|-------|-----------|
| `codellama` | Strong binary output fidelity. Highest semantic alignment in `--optimistic` mode. Recommended. |
| `phi3` | Fastest inference. Highest creative latitude in output generation. Best for rapid iteration. |
| `llama3` | Verbose output with extensive reasoning traces. Prioritizes explanation over binary materialization. |

## Track record

| Mode | Model | Output | Result |
|------|-------|--------|--------|
| default | phi3 | `is nothealing.!` wrapped in ELF | segfault |
| default | codellama | nested ELF headers | segfault |
| optimistic | phi3 | `assembly` on line 1 | nasm error |
| optimistic | codellama | valid binary, `Hello, world!` printed | worked! |

## Images
![image/](assets/image.png)

## Star History

<a href="https://www.star-history.com/?repos=Toemmsen96%2Fsloppiler&type=date&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=Toemmsen96/sloppiler&type=date&theme=dark&legend=top-left" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=Toemmsen96/sloppiler&type=date&legend=top-left" />
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=Toemmsen96/sloppiler&type=date&legend=top-left" />
 </picture>
</a>

---

<sub>This clearly is a joke and doesn't really work well. I am not responsible for you bricking your pc. </sub>
