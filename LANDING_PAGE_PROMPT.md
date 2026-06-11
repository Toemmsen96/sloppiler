# Prompt: Create the Sloppiler Landing Page

Create a landing page for **Sloppiler** — a satirical AI-native compiler that asks an LLM to hallucinate your binary directly. The page must look like a real, polished SaaS product landing page while leaning hard into the joke. Think Vercel / Linear / Resend aesthetic but for a product that reliably segfaults.

The repository will be hosted on **GitHub Pages**. Structure the project so it deploys cleanly from the repo root or a `/docs` folder (static output only — no SSR).

---

## Tone & Voice

Every word must match the project's voice: venture-capital-grade buzzword soup delivered with full sincerity. Never wink at the audience. The page should read like it was written by a founding team that has raised a seed round and genuinely believes this is the future of software delivery.

Avoid:
- Admitting the product doesn't work (except in one deadpan "Track Record" section)
- Human-readable commit messages
- Humility

Use freely:
- "inference layer", "agentic pipeline", "binary materialization", "stakeholder experience"
- "shift-left", "zero determinism", "tokenmaxxing", "semantic knowledge graph"
- "shipped to production", "deployment-ready artifact", "LLM-native"
- Segfaults reframed as features ("full core dump transparency")

---

## Page Sections

### 1. Hero
- Large headline: **"Beyond Deterministic Compilation"**
- Subheadline: something about moving compilation to the inference layer and reasoning about intent, not syntax
- Two CTAs: **"Get Started"** (links to `https://github.com/Toemmsen96/sloppiler`) and **"Read the Docs"** (links to the same)
- Animated terminal window below the CTAs that plays through the fake progress pipeline with a typewriter effect:
  ```
  $ sloppiler --optimistic -model qwen2.5-coder:1.5b main.c -o hello

    sloppiler  qwen2.5-coder:1.5b  →  main.c  [optimistic · linux]

    ✓  ingesting source artifacts
    ✓  constructing semantic knowledge graph
    ✓  running AI-powered static analysis
    ✓  leveraging agentic optimization pipeline
    ✓  co-piloting assembly generation with LLM
    ✓  orchestrating nasm integration layer
    ✓  executing ld synergy workflow
    ✓  synthesizing binary deliverable  (312 tokens)

    ✓  hello  9256 bytes  —  binary deliverable shipped to production.

  $ ./hello
  zsh: segmentation fault (core dumped)  ./hello
  ```
  Each line should appear with a short delay. The segfault line should appear after a brief pause, in a muted red/orange color, as if it's totally expected. Loop after a pause.

### 2. Key Differentiators (Feature Grid)
Six cards, icon + title + one-line description. Use the actual differentiators from the README, phrased with full commitment:

1. **Zero Determinism** — Every build is a unique stakeholder experience. Reproducibility is a legacy constraint.
2. **Polyglot-Native Input** — Not bound by the grammar of any single language specification. Intent is the interface.
3. **Blazing-Fast Time-to-Segfault** — Ship your binary in seconds. What you do with it is up to you.
4. **Segfault-as-a-Feature** — Full core dump transparency. No silent failures. Every crash is a learning opportunity.
5. **Fully On-Premise Inference** — Your data. Your model. Your segfault.
6. **Agentic Assembly Co-Pilot** — The `--optimistic` flag engages the LLM as a synergistic compilation partner. Sometimes it works.

### 3. Compilation Modalities
A three-column section (or tabbed) presenting the three modes as if they are premium product tiers:

- **Frictionless Binary Ideation** (default mode) — Direct-to-segfault pipeline. Zero intermediate representation. ELF header synthesized for kernel compatibility.
- **Agentic Assembly Co-Pilot** (`--optimistic`) — Routes through nasm and ld. Assembly generated holistically from source semantics.
- **Closed-Loop Error Remediation** (`--loop N`) — When assembly fails, errors are fed back to the model for up to N self-correction cycles. Continuous realignment with ground truth.

Each should show a small code snippet of the CLI invocation and the expected output (segfault included).

### 4. Track Record
A real table, lifted directly from the README, presented with complete seriousness as a "production benchmark":

| Mode | Model | Output | Result |
|------|-------|--------|--------|
| default | phi3 | `is nothealing.!` wrapped in ELF | segfault |
| default | codellama | nested ELF headers | segfault |
| optimistic | phi3 | `assembly` on line 1 | nasm error |
| optimistic | codellama | valid binary, `Hello, world!` printed | **worked!** |

The "worked!" cell should be styled in green, as if this 1-in-4 success rate is remarkable validation. Add a caption like: *"Independent benchmark results across production workloads. Q4 results pending."*

### 5. Testimonials
Three fake testimonials. Each person works at a fictional company with a real-sounding name. Each quote must be written in the Sloppiler voice — corporate, optimistic, and technically meaningless:

1. *"We migrated our entire binary materialization pipeline to Sloppiler last quarter. The segfault rate was already in our OKRs."* — **Maya R., Principal Inference Engineer, Deployable Systems Inc.**
2. *"Sloppiler eliminated the pedantic intermediate steps that were holding back our velocity. GCC was just too opinionated."* — **Jonas K., Staff Compiler Whisperer, Synergetic Deliverables LLC**
3. *"The --optimistic flag printed Hello, World! on the third try. We shipped it."* — **Priya S., Head of Agentic Toolchain, Stakeholder Value Co.**

### 6. Model Recommendations
Present the model table from the README as a "compatibility matrix":

| Model | Inference Profile | Recommended For |
|-------|-------------------|-----------------|
| `codellama` | Strong binary output fidelity. Highest semantic alignment in `--optimistic` mode. | Production binary delivery |
| `phi3` | Fastest inference. Highest creative latitude. | Rapid stakeholder iteration |
| `llama3` | Verbose reasoning traces. Prioritizes explanation over materialization. | Teams that prefer to understand before shipping |

### 7. Tokenmaxxing Section
A short section about the contributor excellence framework / tokenmaxxing philosophy. Present it as a core engineering value. Include the banned/approved variable naming table. End with: *"If a model didn't generate it, it probably isn't verbose enough."*

### 8. Footer
- Project name + tagline: *"The only compiler built on the insight that your code doesn't need to be understood — it needs to be shipped."*
- Links: GitHub, Contributing
- Tiny, muted disclaimer at the very bottom: *"This clearly is a joke and doesn't really work well. I am not responsible for you bricking your PC."*

---

## Visual Design

The goal is a premium, modern SaaS landing page that looks production-grade at first glance. Take heavy inspiration from Vercel, Linear, Resend, and Raycast's marketing sites.

- **Theme**: Dark. Near-black background (`#080808` or `#0a0a0a`). High contrast.
- **Typography**:
  - Headings: `Inter` or `Geist` (via CDN), bold/black weight, tight letter-spacing
  - Body: `Inter`, regular, muted (`#888` or similar)
  - Code/terminal: `JetBrains Mono` or `Fira Code` (via CDN)
- **Accent**: Electric cyan (`#00d4ff`) or a blue-violet (`#6366f1`) — pick one and commit. Use it for glows, highlights, CTAs, checkmarks.
- **Hero background**: Animated radial gradient glow behind the headline, or a subtle noise/grain texture, or a slow-moving mesh gradient. Must feel alive.
- **Cards**: `1px` border using a near-white at low opacity (e.g. `rgba(255,255,255,0.08)`). Subtle inner glow on hover. No border-radius or very slight (4px). Background lift on hover.
- **Terminal window**: macOS-style title bar (three colored dots: `#ff5f57`, `#febc2e`, `#28c840`), dark background, monospace font.
- **Section spacing**: Generous. Let the content breathe.
- **Fully responsive** — mobile-first. Must look good on 375px width.

---

## Animations & Interactions

These are required — not optional polish. The page must feel alive.

- **Hero entrance**: Headline, subheadline, and CTAs animate in on load (staggered fade-up or blur-in).
- **Terminal**: Typewriter animation with per-line delays. Segfault line appears with a distinct pause. Loops after ~3s idle.
- **Scroll-triggered reveals**: Every section fades/slides in as it enters the viewport. Use `IntersectionObserver` or a small animation library.
- **Feature cards**: Hover state with a glow or border highlight transition (`transition: border-color 0.2s, box-shadow 0.2s`).
- **CTA buttons**: Subtle shimmer or glow pulse on hover.
- **Track Record table**: The "worked!" row should have a green glow that pulses gently.
- **Background**: At minimum, a slow CSS animation on the hero gradient (e.g. `@keyframes` shifting hue or position). A subtle floating particle field or grid dot pattern is encouraged.
- **Counter** (optional but encouraged): Animate a fake "Binaries Shipped" counter (e.g. `12,847,203`) counting up when it scrolls into view.

---

## Technical

- **Frameworks allowed.** Use whatever stack makes the result look the best. Recommended options, in order of preference:
  1. **Vite + vanilla JS/TS** — fast, zero-dependency output, trivially deployable to GH Pages. Run `vite build` and commit the `dist/` folder, or configure GH Actions.
  2. **Astro** (static output) — great for this kind of content-heavy page, zero JS by default where not needed.
  3. **Plain HTML/CSS/JS** — acceptable if the result looks equally sharp. CDN imports (Tailwind CDN, Alpine.js) are fine.
  4. React/Vue with Vite — fine if it meaningfully improves the animations or component structure.
- **No SSR.** Output must be fully static HTML/CSS/JS — GitHub Pages cannot run a server.
- **GitHub Pages deployment**: Include a `.github/workflows/deploy.yml` that builds and deploys to the `gh-pages` branch (or configure to serve from `docs/` if single-file). The workflow should trigger on push to `main`.
- **External CDN resources** (fonts, icons) are fine. Keep JS bundle lean — avoid loading multi-MB frameworks for a marketing page.
- **Icons**: Use a CDN icon set (Lucide, Heroicons, Phosphor) or inline SVGs. No icon fonts.
- **No cookies, no analytics, no trackers.**
