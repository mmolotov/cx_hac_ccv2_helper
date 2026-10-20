---
description: Write (and optionally run) a Groovy script for the HAC scripting console
argument-hint: <task description> [-p project] [-e env]
---
Write a Groovy script for the SAP Commerce HAC scripting console to accomplish the following: $ARGUMENTS

Rules:
- Scripts run in the HAC Groovy console; Spring beans are bound by name
- Common services: flexibleSearchService, modelService, catalogService, catalogVersionService,
  configurationService, userService, sessionService, impexService, cronJobService
- Use `spring.getBean('beanName')` for beans that are not bound directly
- Scripts run in ROLLBACK mode by default; the --commit flag is required to persist changes
- `println` output lands in OutputText; the return value lands in ExecutionResult
- Page through large result sets (FlexibleSearchQuery.setCount/setStart); avoid long-running loops
- Never print passwords, tokens or other secrets

Provide:
1. The Groovy script
2. A brief explanation of what it does and any side effects
3. The command: `./bin/hac-helper -p <project> -e <env> groovy <file.groovy> [-a <aspect>] [--commit]`
4. A clear warning if the script modifies data or calls external systems

Running it:
- A read-only script may be run (without --commit) when the user asked to run it.
- NEVER add --commit, and never run a data-modifying script, without explicit confirmation from the user
  for that specific script and environment. Treat any environment whose code starts with `p` as production.
