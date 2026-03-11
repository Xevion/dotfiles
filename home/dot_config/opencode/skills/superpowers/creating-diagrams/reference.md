# Mermaid Syntax Reference

Extended syntax for the five core diagram types. See `SKILL.md` for minimal examples and when to use each type.

---

## Flowchart

### Node shapes

```mermaid
flowchart TD
    A[Rectangle]
    B(Rounded)
    C{Diamond — decision}
    D([Stadium — terminal])
    E[(Database)]
    F((Circle))
    G>Asymmetric]
```

### Edges

```mermaid
flowchart LR
    A --> B          %% Arrow
    A --- B          %% Line, no arrow
    A -.-> B         %% Dotted arrow
    A ==> B          %% Thick arrow
    A -- label --> B %% Labeled arrow
    A -->|label| B   %% Alternate label syntax
```

### Subgraphs

```mermaid
flowchart TD
    subgraph Frontend
        A[Browser] --> B[React App]
    end
    subgraph Backend
        C[API Server] --> D[(Database)]
    end
    B --> C
```

Subgraph IDs can be used as edge targets: `Frontend --> Backend`.

### Direction per subgraph

```mermaid
flowchart LR
    subgraph top["Top section"]
        direction TB
        A --> B
    end
```

---

## Sequence Diagram

### Participant types

```mermaid
sequenceDiagram
    participant A as Actor
    actor U as User
    participant S as Service
    participant DB as Database
```

Use `actor` for human participants, `participant` for systems.

### Message types

```mermaid
sequenceDiagram
    A->>B: Synchronous call
    A-->>B: Dashed reply
    A-)B: Async (no wait)
    A-xB: Lost message
```

### Activation bars, notes, loops

```mermaid
sequenceDiagram
    participant C as Client
    participant S as Server

    C->>+S: Request
    Note right of S: Processing...
    S-->>-C: Response

    loop Retry
        C->>S: Retry request
        S-->>C: Response
    end

    alt Success
        C->>S: Confirm
    else Failure
        C->>S: Abort
    end
```

`+` activates the lifeline, `-` deactivates it.

### Fragments reference

| Fragment | Purpose |
| --- | --- |
| `loop <label>` | Repetition |
| `alt <condition>` / `else` | Conditional branches |
| `opt <condition>` | Optional block |
| `par` / `and` | Parallel execution |
| `critical` / `option` | Critical region |
| `break <condition>` | Break out of loop |

---

## Class Diagram

### Visibility modifiers

| Symbol | Meaning |
| --- | --- |
| `+` | Public |
| `-` | Private |
| `#` | Protected |
| `~` | Package/internal |

### Relationships

```mermaid
classDiagram
    ClassA <|-- ClassB       %% Inheritance
    ClassA *-- ClassC       %% Composition
    ClassA o-- ClassD       %% Aggregation
    ClassA --> ClassE       %% Association
    ClassA ..> ClassF       %% Dependency
    ClassA ..|> ClassG      %% Realization (implements interface)
    ClassA -- ClassH        %% Link (solid, no direction)
```

### Labels and cardinality

```mermaid
classDiagram
    Order "1" --> "many" LineItem : contains
    Customer "1" --> "0..*" Order : places
```

### Generics

```mermaid
classDiagram
    class Container~T~ {
        +add(item T) void
        +get() T
    }
```

---

## State Diagram

### Composite states

```mermaid
stateDiagram-v2
    state Processing {
        [*] --> Validating
        Validating --> Executing
        Executing --> [*]
    }
    [*] --> Idle
    Idle --> Processing : submit
    Processing --> Done : success
    Processing --> Error : failure
    Done --> [*]
```

### Concurrency (parallel states)

```mermaid
stateDiagram-v2
    state Fork <<fork>>
    state Join <<join>>

    [*] --> Fork
    Fork --> BranchA
    Fork --> BranchB
    BranchA --> Join
    BranchB --> Join
    Join --> [*]
```

### Notes

```mermaid
stateDiagram-v2
    Idle --> Running
    note right of Idle
        System waits for input here
    end note
```

---

## ER Diagram

### Relationship notation

```
||--||   Exactly one to exactly one
||--o{   One to zero or more
||--|{   One to one or more
o{--o{   Zero or more to zero or more
```

### Full attribute syntax

```mermaid
erDiagram
    PRODUCT {
        int id PK
        string name
        float price
        int categoryId FK "links to CATEGORY"
    }
    CATEGORY {
        int id PK
        string name
    }
    ORDER_LINE {
        int orderId FK
        int productId FK
        int quantity
    }
    PRODUCT }o--|| CATEGORY : "belongs to"
    ORDER_LINE }|--|| PRODUCT : "references"
```

Supported attribute types: `int`, `string`, `float`, `boolean`, `date`, `datetime`. These are display-only — not enforced.

---

## Styling

Mermaid supports inline `style` and `classDef` for node colors. Use sparingly — diagrams should communicate through structure, not decoration.

```mermaid
flowchart TD
    A[Normal]
    B[Warning]
    C[Error]

    classDef warn fill:#fff3cd,stroke:#856404,color:#856404
    classDef error fill:#f8d7da,stroke:#842029,color:#842029

    class B warn
    class C error
```

Avoid per-node `style` inline declarations — `classDef` is easier to maintain when multiple nodes share a style.
