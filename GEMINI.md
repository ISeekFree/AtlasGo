# Gemini CLI Instructions

Follow [AGENTS.md](AGENTS.md) as the canonical coding guide. Consult [Architecture.md](Architecture.md) for flows and [README.md](README.md) for public SDK usage.

Do not couple optional integration modules. Configuration loaders are conveniences and must coexist with manual MongoDB, Redis, and gRPC construction. Validate broad changes with `./build.sh`.
