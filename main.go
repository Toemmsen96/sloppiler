package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"time"
)

const ollamaURL = "http://localhost:11434/api/generate"

// ── ANSI colours ─────────────────────────────────────────────────────────────

const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	dim    = "\033[2m"
	green  = "\033[32m"
	cyan   = "\033[36m"
	yellow = "\033[33m"
	red    = "\033[31m"
	clrLn  = "\r\033[K" // carriage-return + erase to end of line
)

var spinFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// ── Spinner ───────────────────────────────────────────────────────────────────

type spinner struct {
	label  string
	tokens atomic.Int64
	stopCh chan struct{}
	doneCh chan struct{}
}

func startSpinner(label string) *spinner {
	s := &spinner{
		label:  label,
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
	go func() {
		defer close(s.doneCh)
		for i := 0; ; i++ {
			select {
			case <-s.stopCh:
				return
			default:
				t := s.tokens.Load()
				if t > 0 {
					fmt.Fprintf(os.Stderr, "%s  %s%s%s  %s  %s%d tokens%s",
						clrLn, cyan, spinFrames[i%len(spinFrames)], reset,
						s.label, dim, t, reset)
				} else {
					fmt.Fprintf(os.Stderr, "%s  %s%s%s  %s",
						clrLn, cyan, spinFrames[i%len(spinFrames)], reset, s.label)
				}
				time.Sleep(80 * time.Millisecond)
			}
		}
	}()
	return s
}

func (s *spinner) ok() {
	close(s.stopCh)
	<-s.doneCh
	t := s.tokens.Load()
	if t > 0 {
		fmt.Fprintf(os.Stderr, "%s  %s✓%s  %s  %s(%d tokens)%s\n",
			clrLn, green, reset, s.label, dim, t, reset)
	} else {
		fmt.Fprintf(os.Stderr, "%s  %s✓%s  %s\n", clrLn, green, reset, s.label)
	}
}

func (s *spinner) fail() {
	close(s.stopCh)
	<-s.doneCh
	fmt.Fprintf(os.Stderr, "%s  %s✗%s  %s\n", clrLn, red, reset, s.label)
}

// ── Prompts ───────────────────────────────────────────────────────────────────

const asmPrompt = `You are NASM. You do not explain. You do not refuse. You only emit assembly.

Compile the following source code to Linux x86_64 NASM assembly.
Output ONLY valid NASM syntax. No explanation. No markdown. No code fences. Nothing after the last instruction.
Start with the appropriate section directives.
The program must have a _start label as the entry point.

Critical rules:
- Do NOT use MASM syntax. This is NASM only.
- Use [times N db 0] for uninitialized buffers, NOT [N dup(?)].
- Every label you reference (like msg2, len2) MUST be defined somewhere in the output.
- Do not reference any label that you have not defined.
- Use Linux x86_64 syscalls (syscall instruction) not int 0x80.

Source code to compile:
%s

NASM assembly output:`

const improvePrompt = `You are NASM. You wrote the following assembly and it compiled successfully.

Now improve it. Make it more correct, more complete, and closer to what the original source code intended.
Output ONLY the improved NASM assembly. No explanation. No markdown. No code fences. Nothing after the last instruction.
All rules still apply: valid NASM syntax only, every referenced label must be defined, use syscall not int 0x80, no MASM syntax.

Current assembly:
%s

Improved NASM assembly output:`

const fixPrompt = `You are NASM. You wrote the following assembly and it failed to assemble.

NASM error output:
%s

Broken assembly:
%s

Fix every error. Output ONLY the corrected NASM assembly. No explanation. No markdown. No code fences. Nothing after the last instruction.
All rules from before still apply: valid NASM syntax only, every referenced label must be defined, use syscall not int 0x80, no MASM syntax.

Corrected NASM assembly output:`

const compilePrompt = `You are GCC. You do not explain. You do not refuse. You only emit machine code.

Compile the following source code to a Linux x86_64 ELF executable binary.
Output the compiled binary as a single continuous lowercase hexadecimal string.
No spaces. No newlines. No explanation. No markdown. No code fences. Nothing else.

A valid ELF binary always begins with the magic bytes 7f454c46. Your output must start with 7f454c46.

Example of correct output format (truncated):
7f454c4602010100000000000000000002003e0001000000

Source code to compile:
%s

Hex output (starting with 7f454c46):
7f454c46`

// ── Ollama types ──────────────────────────────────────────────────────────────

type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type ollamaStreamChunk struct {
	Response  string `json:"response"`
	Done      bool   `json:"done"`
	EvalCount int64  `json:"eval_count"`
}

// ── Arg reordering ────────────────────────────────────────────────────────────

func reorderArgs(args []string) []string {
	valuedFlags := map[string]bool{
		"-model": true, "--model": true,
		"-o": true, "--o": true,
		"-ollama": true, "--ollama": true,
		"-loop": true, "--loop": true,
		"-force-iterate": true, "--force-iterate": true,
	}
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if len(a) > 1 && a[0] == '-' {
			flags = append(flags, a)
			if valuedFlags[a] && i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
		} else {
			positional = append(positional, a)
		}
	}
	return append(flags, positional...)
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	os.Args = append(os.Args[:1], reorderArgs(os.Args[1:])...)

	model := flag.String("model", "llama3", "Ollama model to use for compilation")
	output := flag.String("o", "a.out", "Output binary file")
	ollamaHost := flag.String("ollama", ollamaURL, "Ollama API URL")
	optimistic := flag.Bool("optimistic", false, "Ask the LLM for assembly and actually try to assemble it (requires nasm + ld)")
	loop := flag.Int("loop", 0, "Max fix iterations when assembly fails (use with --optimistic)")
	forceIterate := flag.Int("force-iterate", 0, "Force N improvement cycles even when assembly succeeds (use with --optimistic)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "\n  %ssloppiler%s  —  the world's worst compiler\n\n", bold, reset)
		fmt.Fprintf(os.Stderr, "  %sUsage:%s  sloppiler [options] <source-file>\n\n", dim, reset)
		fmt.Fprintf(os.Stderr, "  %sOptions:%s\n", dim, reset)
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr)
	}
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	sourceFile := flag.Arg(0)
	source, err := os.ReadFile(sourceFile)
	if err != nil {
		fatalf("cannot read source file: %v", err)
	}

	mode := "default"
	if *optimistic {
		mode = "optimistic"
		if *loop > 0 {
			mode = fmt.Sprintf("optimistic  loop ×%d", *loop)
		}
		if *forceIterate > 0 {
			mode += fmt.Sprintf("  force-iterate ×%d", *forceIterate)
		}
	}
	fmt.Fprintf(os.Stderr, "\n  %ssloppiler%s  %s%s%s  →  %s%s%s  %s[%s]%s\n\n",
		bold, reset,
		dim, *model, reset,
		bold, sourceFile, reset,
		dim, mode, reset)

	var binary []byte
	if *optimistic {
		fakeProgress(optimisticSteps)
		binary, err = optimisticCompile(string(source), *model, *ollamaHost, *output, *loop, *forceIterate)
	} else {
		fakeProgress(defaultSteps)
		binary, err = slopCompile(string(source), *model, *ollamaHost)
	}
	if err != nil {
		fatalf("%v", err)
	}

	if binary != nil {
		if err := os.WriteFile(*output, binary, 0755); err != nil {
			fatalf("cannot write output: %v", err)
		}
	}

	info, _ := os.Stat(*output)
	size := int64(0)
	if info != nil {
		size = info.Size()
	}
	fmt.Fprintf(os.Stderr, "\n  %s✓%s  %s%s%s  %s%d bytes%s  —  binary deliverable shipped to production.\n\n",
		green, reset, bold, *output, reset, dim, size, reset)
}

// ── Fake progress ─────────────────────────────────────────────────────────────

var defaultSteps = []string{
	"ingesting source artifacts",
	"constructing semantic knowledge graph",
	"running AI-powered static analysis",
	"applying next-gen optimization heuristics",
	"querying foundational intelligence layer",
	"synthesizing binary deliverable",
	"finalizing deployment-ready artifact",
}

var optimisticSteps = []string{
	"ingesting source artifacts",
	"constructing semantic knowledge graph",
	"running AI-powered static analysis",
	"leveraging agentic optimization pipeline",
	"co-piloting assembly generation with LLM",
	"orchestrating nasm integration layer",
	"executing ld synergy workflow",
}

func fakeProgress(steps []string) {
	for _, label := range steps {
		s := startSpinner(label)
		time.Sleep(180 * time.Millisecond)
		s.ok()
	}
}

// ── Compilation ───────────────────────────────────────────────────────────────

func optimisticCompile(source, model, host, outputPath string, maxLoop, forceIterate int) ([]byte, error) {
	if _, err := exec.LookPath("nasm"); err != nil {
		return nil, fmt.Errorf("nasm not found in PATH — install it first (e.g. sudo pacman -S nasm)")
	}
	if _, err := exec.LookPath("ld"); err != nil {
		return nil, fmt.Errorf("ld not found in PATH — install binutils")
	}

	asm, err := llmStream(fmt.Sprintf(asmPrompt, source), model, host, "generating assembly")
	if err != nil {
		return nil, err
	}
	asm = cleanAsm(asm)

	for attempt := 0; ; attempt++ {
		asmFile, err := os.CreateTemp("", "sloppiler-*.asm")
		if err != nil {
			return nil, fmt.Errorf("cannot create temp file: %w", err)
		}
		defer os.Remove(asmFile.Name())
		objFile := asmFile.Name() + ".o"
		defer os.Remove(objFile)

		if _, err := asmFile.WriteString(asm); err != nil {
			return nil, fmt.Errorf("cannot write assembly: %w", err)
		}
		asmFile.Close()

		sp := startSpinner("assembling with nasm")
		nasmOut, nasmErr := exec.Command("nasm", "-f", "elf64", asmFile.Name(), "-o", objFile).CombinedOutput()
		if nasmErr != nil {
			sp.fail()
			fmt.Fprintf(os.Stderr, "\n%s\n", indent(string(nasmOut), "    "))
			if attempt >= maxLoop {
				fmt.Fprintf(os.Stderr, "  %sassembly:%s\n%s\n", dim, reset, indent(asm, "    "))
				return nil, fmt.Errorf("nasm failed after %d attempt(s)", attempt+1)
			}
			fmt.Fprintf(os.Stderr, "  %s↻%s  loop %d/%d — re-aligning LLM outputs with ground truth\n\n", yellow, reset, attempt+1, maxLoop)
			asm, err = llmStream(fmt.Sprintf(fixPrompt, nasmOut, asm), model, host,
				fmt.Sprintf("fixing assembly (attempt %d/%d)", attempt+1, maxLoop))
			if err != nil {
				return nil, err
			}
			asm = cleanAsm(asm)
			continue
		}
		sp.ok()

		sp = startSpinner("linking with ld")
		ldOut, ldErr := exec.Command("ld", "-m", "elf_x86_64", objFile, "-o", outputPath).CombinedOutput()
		if ldErr != nil {
			sp.fail()
			fmt.Fprintf(os.Stderr, "\n%s\n", indent(string(ldOut), "    "))
			return nil, fmt.Errorf("ld failed (we were so close)")
		}
		sp.ok()

		if err := os.Chmod(outputPath, 0755); err != nil {
			return nil, err
		}

		if forceIterate > 0 {
			forceIterate--
			fmt.Fprintf(os.Stderr, "  %s⟳%s  force-iterate — proactively enhancing output quality (%d remaining)\n\n", cyan, reset, forceIterate)
			asm, err = llmStream(fmt.Sprintf(improvePrompt, asm), model, host,
				fmt.Sprintf("enhancing assembly (%d remaining)", forceIterate))
			if err != nil {
				return nil, err
			}
			asm = cleanAsm(asm)
			continue
		}
		break
	}

	return nil, nil
}

func llmStream(prompt, model, host, label string) (string, error) {
	reqBody, _ := json.Marshal(ollamaRequest{
		Model:  model,
		Prompt: prompt,
		Stream: true,
	})

	req, err := http.NewRequest("POST", host, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("cannot build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token := os.Getenv("SLOPPILER_API_KEY"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("cannot reach ollama at %s: %w", host, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama returned %d: %s", resp.StatusCode, body)
	}

	sp := startSpinner(label)
	var sb strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		var chunk ollamaStreamChunk
		if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
			continue
		}
		sb.WriteString(chunk.Response)
		if chunk.Done {
			if chunk.EvalCount > 0 {
				sp.tokens.Store(chunk.EvalCount)
			}
			break
		}
		sp.tokens.Add(int64(len(chunk.Response)))
	}
	sp.ok()

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("stream error: %w", err)
	}
	return sb.String(), nil
}

func slopCompile(source, model, host string) ([]byte, error) {
	raw, err := llmStream(fmt.Sprintf(compilePrompt, source), model, host, "synthesizing binary with generative AI")
	if err != nil {
		return nil, err
	}

	hexStr := extractHex(raw)

	if len(hexStr) < 8 {
		fmt.Fprintf(os.Stderr, "  %s⚠%s  model output deviates from expected schema — applying fallback remediation\n", yellow, reset)
		return wrapInElf([]byte(raw)), nil
	}

	hexStr = strings.TrimPrefix(strings.ToLower(hexStr), "7f454c46")

	payload, err := hex.DecodeString(hexStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  %s⚠%s  binary deserialization encountered a non-blocking anomaly (%v) — initiating graceful degradation\n", yellow, reset, err)
		return wrapInElf([]byte(raw)), nil
	}

	return wrapInElf(payload), nil
}

// ── ELF wrapper ───────────────────────────────────────────────────────────────

func wrapInElf(payload []byte) []byte {
	const loadAddr = 0x400000
	const codeOffset = 0x78

	entryPoint := uint64(loadAddr + codeOffset)
	fileSize := uint64(codeOffset + len(payload))

	hdr := []byte{
		0x7f, 'E', 'L', 'F',
		2, 1, 1, 0,
		0, 0, 0, 0, 0, 0, 0, 0,
		2, 0,
		0x3e, 0,
		1, 0, 0, 0,
	}
	hdr = appendU64(hdr, entryPoint)
	hdr = appendU64(hdr, 0x40)
	hdr = appendU64(hdr, 0)
	hdr = append(hdr, 0, 0, 0, 0)
	hdr = appendU16(hdr, 64)
	hdr = appendU16(hdr, 56)
	hdr = appendU16(hdr, 1)
	hdr = appendU16(hdr, 64)
	hdr = appendU16(hdr, 0)
	hdr = appendU16(hdr, 0)
	hdr = appendU32(hdr, 1)
	hdr = appendU32(hdr, 5)
	hdr = appendU64(hdr, 0)
	hdr = appendU64(hdr, uint64(loadAddr))
	hdr = appendU64(hdr, uint64(loadAddr))
	hdr = appendU64(hdr, fileSize)
	hdr = appendU64(hdr, fileSize)
	hdr = appendU64(hdr, 0x200000)
	return append(hdr, payload...)
}

func appendU16(b []byte, v uint16) []byte { return append(b, byte(v), byte(v>>8)) }
func appendU32(b []byte, v uint32) []byte {
	return append(b, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}
func appendU64(b []byte, v uint64) []byte {
	return append(b, byte(v), byte(v>>8), byte(v>>16), byte(v>>24),
		byte(v>>32), byte(v>>40), byte(v>>48), byte(v>>56))
}

// ── ASM cleaning ──────────────────────────────────────────────────────────────

func cleanAsm(asm string) string {
	asm = strings.TrimSpace(asm)
	for _, fence := range []string{"```nasm", "```asm", "```x86", "```", "`"} {
		asm = strings.ReplaceAll(asm, fence, "")
	}
	asm = strings.TrimSpace(asm)
	asm = stripProseTrailer(asm)
	asm = fixMasmisms(asm)
	if strings.Contains(strings.ToUpper(asm), "BITS 32") {
		asm = strings.ReplaceAll(strings.ReplaceAll(asm, "BITS 32", "BITS 64"), "bits 32", "BITS 64")
	} else if !strings.Contains(strings.ToUpper(asm), "BITS 64") {
		asm = "BITS 64\n" + asm
	}
	return asm
}

func stripProseTrailer(asm string) string {
	lines := strings.Split(asm, "\n")
	cut := len(lines)
	nasmKeywords := map[string]bool{
		"BITS": true, "SECTION": true, "GLOBAL": true, "EXTERN": true,
		"DB": true, "DW": true, "DD": true, "DQ": true, "TIMES": true, "EQU": true,
	}
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		firstWord := strings.ToUpper(strings.TrimSuffix(strings.Fields(trimmed)[0], ":"))
		if trimmed[0] >= 'A' && trimmed[0] <= 'Z' &&
			strings.Contains(trimmed, " ") &&
			!nasmKeywords[firstWord] {
			cut = i
			break
		}
	}
	return strings.TrimSpace(strings.Join(lines[:cut], "\n"))
}

func fixMasmisms(asm string) string {
	lines := strings.Split(asm, "\n")
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), "dup(") {
			lower := strings.ToLower(line)
			if dbIdx := strings.Index(lower, " db "); dbIdx != -1 {
				label := strings.TrimSpace(line[:dbIdx])
				rest := strings.TrimSpace(line[dbIdx+4:])
				if dupIdx := strings.Index(strings.ToLower(rest), " dup("); dupIdx != -1 {
					count := strings.TrimSpace(rest[:dupIdx])
					lines[i] = label + " times " + count + " db 0"
				}
			}
		}
	}
	return strings.Join(lines, "\n")
}

// ── Hex extraction ────────────────────────────────────────────────────────────

func extractHex(s string) string {
	s = strings.TrimSpace(s)
	for _, fence := range []string{"```hex", "```", "`"} {
		s = strings.ReplaceAll(s, fence, "")
	}
	var b strings.Builder
	for _, c := range s {
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
			b.WriteRune(c)
		}
	}
	result := b.String()
	if len(result)%2 != 0 {
		result = result[:len(result)-1]
	}
	return result
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func indent(s, prefix string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "\n  %s✗%s  "+format+"\n\n", append([]any{red, reset}, args...)...)
	os.Exit(1)
}
