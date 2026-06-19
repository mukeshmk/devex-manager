# DevEx Manager: Technical Workflows

## 1. git wt — Worktree Management

```mermaid
graph LR
    subgraph Standard [Standard Git]
    direction TB
    S1[Clone] --> S2[Work on A]
    S2 --> S3{Switch Task?}
    S3 -- "Yes" --> S4[git stash]
    S4 --> S5[git checkout B]
    S5 --> S6[Fix Bug]
    S6 --> S7[git checkout A]
    S7 --> S8[git stash pop]
    end

    Standard === VS(( VS )) === Worktree

    subgraph Worktree [DevEx Worktree]
    direction TB
    W1[git wt clone] --> W2[folder: /main]
    W2 --> W3[git wt add A]
    W3 --> W4[folder: /A]
    W4 --> W5{Switch Task?}
    W5 -- "Yes" --> W6[git wt add B]
    W6 --> W7[folder: /B]
    W7 --> W8[Just cd ../A]
    end

    classDef blue fill:none,stroke:#38b6ff,stroke-width:2px
    classDef green fill:none,stroke:#76d275,stroke-width:2px
    classDef yellow fill:none,stroke:#ffcc80,stroke-width:2px
    classDef white fill:none,stroke:#fff,stroke-width:2px

    class W1,W2,S1,S2 blue
    class W3,W4,W7,W8 green
    class S3,W5,S4,S5,S6,S7,S8,W6 yellow
    class VS white
    
    style S4 stroke-dasharray: 5 5
    style W8 stroke-width:4px
```

**Key Benefit:** Eliminates `git stash` and context switching. Each branch has its own directory and persistent IDE state.

### Configuration Copies Sync

When configuration paths are copied instead of symlinked across worktrees, their states drift over time. The `git wt sync` command solves this by running a bidirectional, file-level synchronization:

```mermaid
flowchart TD
    Run([Run git wt sync]) --> LockCheck{Conflict markers in main?}
    LockCheck -- Yes --> Block[Abort Sync with error]
    LockCheck -- No --> ScanWTs[Scan active worktrees & manifest]
    ScanWTs --> EvalRules{Evaluate rules per file}
    
    EvalRules -- "Untouched everywhere" --> Skip[Skip file]
    EvalRules -- "Modified in Main or single WT" --> SilentlyPropagate[Propagate changes to all worktrees]
    EvalRules -- "Deleted in Main or single WT" --> SilentlyDel[Delete from WTs or prompt to delete/restore]
    EvalRules -- "Conflict (Main vs WT or WT vs WT)" --> PromptUser[Prompt user for choice]
    
    PromptUser -- "Keep Main version" --> OverwriteWT[Overwrite WT with Main version]
    PromptUser -- "Keep WT version" --> OverwriteMain[Overwrite Main and fan out to all WTs]
    PromptUser -- "Resolve Later" --> ConflictMarkers[Create *.devex-conflict-* marker file in main]
    
    OverwriteWT --> RebuildManifest[Rebuild sync manifest]
    OverwriteMain --> RebuildManifest
    ConflictMarkers --> RebuildManifest
    SilentlyPropagate --> RebuildManifest
    SilentlyDel --> RebuildManifest
    Skip --> RebuildManifest
    
    RebuildManifest --> End([Synchronization Complete])
```

---

## 2. dx nb — Notebook Utilities

```mermaid
sequenceDiagram
    participant Dev as Developer
    participant NB as Notebook
    participant Git as Git Index

    Dev->>NB: Experiment & Run
    Note right of NB: 🟡 Dirty (Outputs/Metadata)
    Dev->>NB: dx nb strip
    Note right of NB: 🟢 Clean (Ready to commit)
    Dev->>Git: git add & commit
    Note right of Git: 🟢 Minimal Diffs
    Dev->>NB: dx nb kernel
    Note right of NB: 🔵 Venv Registered
```

**Key Benefit:** Keeps notebook history clean and ensures every worktree has its own dedicated Jupyter kernel.

---

## 3. Auto-Venv — Environment Automation

```mermaid
stateDiagram-v2
    [*] --> Terminal
    Terminal --> CD_Into_Project: cd my-project
    
    state "Auto-Venv Logic" as Hook {
        CD_Into_Project --> CheckVenv: .venv exists?
        CheckVenv --> Activate: Yes
        CheckVenv --> Prompt: No (Manifest found)
        
        Activate --> CheckStale: Merged?
        CheckStale --> Warning: Yes
        CheckStale --> Ready: No
        
        Prompt --> Init: 'y'
        Init --> Activate
    }
    
    Ready --> CD_Out: cd ..
    CD_Out --> Deactivate
    Deactivate --> [*]

    class Terminal,CD_Into_Project,CheckVenv,CheckStale,CD_Out blue
    class Activate,Ready,Init green
    class Prompt,Warning,Deactivate yellow
```

**Key Benefit:** Zero-touch virtual environment management and proactive warnings for merged worktrees.

---

## 4. git ctx — Developer Context Manager

```mermaid
graph TD
    A[git ctx show] --> B{Context file exists?}
    B -- "No" --> C[Initialize .git/info/devex/contexts/branch.md]
    B -- "Yes" --> D[Read & parse markdown checklist]
    C --> D
    D --> E[Print Checklist & Notes to terminal]
    
    F[git ctx done 1] --> G[Locate task in markdown file]
    G --> H[Replace [ ] with [x]]
    H --> E
    
    I[git ctx clean] --> J[Scan contexts folder]
    J --> K{Branch still exists?}
    K -- "No" --> L[List as Orphan]
    K -- "Yes" --> M[Skip]
    L --> N[Prompt to Delete]
    N -- "y" --> O[Remove context file]
```

**Key Benefit:** Keeps task-specific scratchpads and checklist items localized to each branch without polluting git commits or configuration.
