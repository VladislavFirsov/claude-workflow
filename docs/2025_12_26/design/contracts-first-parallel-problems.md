# Contracts-First Parallel: Проблемы и решения

## Контекст

**Проект:** Runtime Layer для агентных LLM-систем (sidecar, Go, LangChain)

**Подход:** Contracts-First Parallel Development
- spec-architect генерирует контракты (interfaces, DTO, errors)
- Простые компоненты → параллельно через Haiku
- Сложные компоненты → Sonnet/Opus
- Codex → только для GitHub code review

**Документ архитектуры:** `docs/2025_12_26/design/runtime-layer-v1-draft.md`

---

## Проблема 1: Потребление токенов ✅ РЕШЕНО

### Суть
При параллельном запуске агентов каждый получает полный system prompt, контракты, контекст. Это дублирование увеличивает стоимость на 80%.

### Решение: Tiered Prompts

```
TIER 1 (800-1,000 токенов) — Haiku
  Структура: <role> + <contract> (inline) + <pattern> (1 пример) + <task> + <success_criteria>
  Для: детерминированная логика, чистые вычисления, stateless

TIER 2 (1,500-2,200 токенов) — Sonnet
  Добавляет: <dependencies> + <business_rules> + <patterns> (2-3) + <edge_cases>
  Для: координация, state management, интеграция компонентов

TIER 3 (3,000-4,500 токенов) — Opus
  Добавляет: <security> + <architecture_context> + <thinking_instruction>
  Для: критичная логика, нельзя ошибиться
```

### Best Practices для промптов (из Anthropic docs)

**Структура:**
- XML-теги: `<role>`, `<contract>`, `<pattern>`, `<task>`, `<success_criteria>`
- Логический порядок: контекст → данные → задача → критерии

**Ключевые принципы:**
1. Smallest high-signal tokens — минимум токенов, максимум пользы
2. Goldilocks zone — не хардкодить логику, но быть specific
3. Inline contracts — передавать в prompt, не читать через Read
4. 1-3 diverse examples — из реального проекта, не laundry list
5. Explicit instructions — "Implement X" не "Can you suggest"
6. Success criteria — testable conditions в каждом промпте
7. Say WHAT to do, not WHAT NOT — с объяснением ПОЧЕМУ

**Исключать из промптов:**
- CLAUDE.md (для основной сессии)
- Workflow инструкции (агент делает одну задачу)
- Laundry list edge cases
- Redundant tool outputs
- Interfaces слоёв без прямой зависимости

### Экономия
- Токены: 50% меньше
- Стоимость vs Sonnet для всех: 79% меньше
- Стоимость vs Opus для всех: 96% меньше

### Sources
- https://www.anthropic.com/engineering/claude-code-best-practices
- https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents
- https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-4-best-practices

---

## Проблема 2: Классификация сложности ✅ РЕШЕНО

### Суть
Как определить: этот компонент для Haiku (TIER 1) или Opus (TIER 3)? Кто решает? На основе чего?

### Решение: Критерии для Runtime Layer

Не слой определяет сложность, а характеристики задачи:

**TIER 1 — HAIKU (детерминированная логика)**
- Чистые вычисления (формулы, калькуляторы)
- Простые трансформации данных
- Stateless операции
- Нет side effects
- Легко покрыть unit тестами
- Формула/алгоритм известен заранее

**TIER 2 — SONNET (координация и state)**
- Управление состоянием
- Координация между компонентами
- Принятие решений на основе состояния
- Обработка edge cases
- Интеграция нескольких компонентов

**TIER 3 — OPUS (критичная логика)**
- Ошибка = деньги клиента (BudgetEnforcer)
- Ошибка = каскадный сбой (CircuitBreaker)
- Ошибка = потеря информации (ContextCompactor)
- Ошибка = некорректное исполнение (Scheduler, ParallelExecutor)
- Интеграция с внешним кодом (Adapters)

### Классификация компонентов Runtime Layer v1

| Компонент | TIER | Обоснование |
|-----------|------|-------------|
| TokenEstimator | 1 | Чистая формула: tokens = f(text) |
| CostCalculator | 1 | Чистая формула: cost = tokens × price |
| UsageTracker | 1 | Простой аккумулятор |
| QueueManager | 2 | State, но простой (in-memory queue) |
| MemoryManager | 2 | Key-value, short-term |
| ContextBuilder | 2 | Сборка по известным правилам |
| ContextRouter | 2 | Передача данных между tasks |
| DependencyResolver | 2 | DAG build and validation |
| **Scheduler** | **3** | Порядок = корректность всей системы |
| **ParallelExecutor** | **3** | Race conditions, bounded concurrency |
| **BudgetEnforcer** | **3** | Ошибка = перерасход денег клиента |
| **ContextCompactor** | **3** | Ошибка = потеря информации |

---

## Проблема 3: Context Sharing ✅ РЕШЕНО

### Суть
Агенты работают параллельно — как им делиться контекстом? Нужно ли вообще?

### Решение: Contracts-First БЕЗ runtime sharing

**Общие типы и интерфейсы в contracts/ закрывают риск расхождений.**
Sharing нужен только если появится реальная общая утилита или протокол сериализации.

### Что определено в contracts (runtime-layer-v1-draft.md)

**Базовые типы:**
```go
type RunID string
type TaskID string
type ModelID string
type TokenCount int64
type Currency string
type RunState int   // enum: Pending, Running, Completed, Failed, Aborted
type TaskState int  // enum: Pending, Ready, Running, Completed, Failed, Skipped
```

**Структуры данных:**
```go
type Run struct { ID, State, Policy, DAG, Tasks, Usage, CreatedAt, UpdatedAt }
type Task struct { ID, State, Inputs, Deps, Outputs, Error, Model, EstimatedUse, ActualUse }
type DAG struct { Nodes, Edges }
type DAGNode struct { ID, Deps, Next, Pending }
type Usage struct { Tokens, Cost }
type Cost struct { Amount, Currency }
type TaskInput struct { Prompt, Inputs, Metadata }
type TaskResult struct { Output, Outputs, Usage, Metadata }
type TaskError struct { Code, Message }
type ContextBundle struct { Messages, Memory, Tools }
type ContextPolicy struct { MaxTokens, Strategy, KeepLastN, TruncateTo }
type RunPolicy struct { TimeoutMs, MaxParallelism, BudgetLimit }
```

**12 интерфейсов по 3 доменам** — полностью определены.

### Правила для агентов

1. **Агенты получают:** только свой interface + нужные типы (inline в промпт)
2. **Агенты НЕ создают:** новые публичные типы, shared helpers, изменения в contracts/
3. **Internal код:** каждый агент пишет в свой пакет (internal/{domain}/{component}.go)
4. **Integration phase:** go build ./... — проверка компиляции, поиск дубликатов

### Когда sharing НЕ нужен

| Сценарий | Sharing? | Решение |
|----------|----------|---------|
| Общие типы (TokenCount) | ❌ | Inline в промпт |
| Общие структуры (Run, Task) | ❌ | Inline в промпт |
| Общие interfaces | ❌ | Inline в промпт |
| Общие errors | ❌ | Inline в промпт |
| Internal helpers | ❌ | Каждый в своём пакете |
| Вызов другого компонента | ❌ | Через interface |

### Когда sharing МОЖЕТ понадобиться (в будущем)

- Общая утилита используется 3+ компонентами
- Протокол сериализации (JSON/Protobuf helpers)
- Общий middleware/interceptor

→ Решение: добавить в contracts/helpers.go или pkg/

---

## Архитектура решения

```
/parallel-dev "Implement Runtime Layer v1"
      │
      ▼
ФАЗА 0: Классификация
      │ Определить компоненты и их TIER
      ▼
ФАЗА 1: Контракты (уже есть в runtime-layer-v1-draft.md)
      │ interfaces/, data structures
      ▼
ФАЗА 2: Реализация (параллельно по TIER)
      │
      ├── TIER 1 (Haiku, параллельно):
      │   ├── TokenEstimator
      │   ├── CostCalculator
      │   └── UsageTracker
      │
      ├── TIER 2 (Sonnet, параллельно):
      │   ├── QueueManager
      │   ├── MemoryManager
      │   ├── ContextBuilder
      │   ├── ContextRouter
      │   └── DependencyResolver
      │
      └── TIER 3 (Opus, последовательно или с review):
          ├── Scheduler
          ├── ParallelExecutor
          ├── BudgetEnforcer
          └── ContextCompactor
      │
      ▼
ФАЗА 3: Интеграция
      │ Проверка компиляции, тесты, рефактор
      ▼
ГОТОВО
```

---

## Следующие шаги

1. [x] Решить проблему 1 (токены) — Tiered Prompts
2. [x] Решить проблему 2 (классификация) — критерии для Runtime
3. [x] Решить проблему 3 (context sharing) — Contracts-First без runtime sharing
4. [x] Создать contracts/ файлы — runtime/contracts/ (5 файлов, компилируется)
5. [x] Создать шаблоны промптов — runtime/prompts/ (tier1.md, tier2.md, tier3.md)
6. [x] Реализовать /parallel-dev команду — .claude/commands/parallel-dev.md + manifest.json
7. [x] PoC: TokenEstimator (TIER 1, Haiku) — runtime/internal/cost/token_estimator.go
8. [x] PoC: Scheduler (TIER 3, Opus) — runtime/internal/orchestration/scheduler.go
9. [x] PoC: QueueManager (TIER 2, Sonnet) — runtime/internal/orchestration/queue_manager.go
10. [x] Реализовать все 12 компонентов Runtime Layer

---

## Финальный отчёт

### Статистика

| Метрика | Значение |
|---------|----------|
| Компоненты | 12/12 |
| Тесты | 195 passed |
| Coverage (context) | 100.0% |
| Coverage (cost) | 98.7% |
| Coverage (orchestration) | 97.8% |
| Race conditions | 0 (verified with -race) |

### Компоненты по TIER

**TIER 1 (Haiku):**
- TokenEstimator ✓
- CostCalculator ✓

**TIER 2 (Sonnet, параллельно):**
- QueueManager ✓
- UsageTracker ✓
- DependencyResolver ✓
- ContextBuilder ✓
- ContextRouter ✓
- MemoryManager ✓

**TIER 3 (Opus, последовательно):**
- Scheduler ✓
- BudgetEnforcer ✓
- ParallelExecutor ✓
- ContextCompactor ✓

### Quality Gates

| Tier | Target | Actual | Status |
|------|--------|--------|--------|
| TIER 1 | ≥90% | 98.7% | ✓ |
| TIER 2 | ≥90% | 97-100% | ✓ |
| TIER 3 | ≥95% | 97-100% | ✓ |