# Agent Notes

`droid` is a self-contained agent for Linux. It is not a generic Agent,
it will have specific tooling for coding with additional
security features missing in modern Agents.
The goal is high-performance, no dependencies, Go codebase.
Modern Agents are built with typescript, node, lots of useless code that make
them heavy, buggy and too generalized, our must be a feather.

## Goals

- Not a generic Agent, we focus on product.
- Easy to install and ru.
- Must be specialized with small offline models like Qwen or deepseek coder.
- Great tooling.
- OPENAI API compatible models support.
- Token fiendly system prompt.
- Skills
- Theming.

## Innovations

- Namespaces (Sandboxes) for running an agent in a safe-space.
- `Filewall`, block the read of files like .env or at least ask the user.
- Self improving agent?
- LLM Wiki integration?
- RAG?

## Quality Rules

- Write the dumbest code that works.
- When working with strings formatting avoid fmt.Snprintf or similar
  since are inefficient, let's use the stack when possibile.
- No god functions, every function must be short with single responsability.
- Keep the implementation small, sharp, easy to understand.
- Try to write elegant code in a state of grace.
  Don't settle for the first thing that comes to mind, try to find the most
  minimal and better working design.
  Don't introduce slop: very fragile code that just patches specific cases,
  dead code, useless code and code ways more complicated of how it should be.
- Comment important inference code where the model mechanics, cache lifetime,
  memory policy, or API orchestration are not obvious from the local code.
- Prefer comments beside the implementation over separate design documents.
- Keep comments instructive and compact:
  explain why a shape, ordering, cache boundary, or memory choice exists.
- Keep public APIs narrow. CLI/server code should not know tensor internals.
- No unreadable variable names like N for Node or M for Model.
