---
description: Write (and optionally run) a FlexibleSearch query against a HAC environment
argument-hint: <task description> [-p project] [-e env]
---
Write a FlexibleSearch query for the following task: $ARGUMENTS

Rules:
- Use {TypeName} for item types, {field} for attributes
- Use the :o suffix for optional (outer) joins / localized attributes: {name[en]:o}
- Use `!` for exact type without subtypes: {Product!}
- Types follow the SAP Commerce type system (GenericItem, Product, Category, Order, CronJob, ...)
- Escape single quotes in literals by doubling them: 'o''brien'
- Prefer ORDER BY on indexed fields; always keep the result bounded (use --max-count)
- For raw SQL use the --sql flag (table names like `products`, columns like `p_code`)
- For a plain count prefer `hac-helper count <Type> --where "..."`

Provide:
1. The FlexibleSearch query
2. A brief explanation of what it does
3. The command: `./bin/hac-helper -p <project> -e <env> flexsearch "<query>" [-f csv] [--max-count N]`

If the user gave a project/environment (or asked to run it), build with `make build` if `bin/hac-helper` is missing,
run the query and summarise the result. FlexibleSearch is read-only, so running it needs no extra confirmation.
