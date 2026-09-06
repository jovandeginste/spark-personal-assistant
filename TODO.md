# Complexity Reduction TODO

- [x] Split `cmd/spark/router.go:routerCmd` into smaller startup helpers.
  - The command currently initializes AI, Matrix, web, mail, goroutines, and shutdown handling inline. Extracting Matrix, web, mail, and signal-wait setup would reduce nesting without changing behavior.

- [x] Remove duplicated Matrix setup between `cmd/spark/router.go` and `cmd/spark/matrix.go`.
  - Matrix client construction, AI data setup, syncer/crypto/chat initialization, greeting, and registration are repeated or closely mirrored. A shared Matrix startup helper would make changes safer.

- [x] Simplify MCP reconnect retry flow in `pkg/app/mcp.go:GetMCPTools` and `pkg/app/mcp.go:ExecuteMCPTool`.
  - Both functions implement “try, force reconnect, try once again”. A small retry helper would reduce repeated error handling and logging.

- [x] Extract MCP transport construction from `pkg/app/mcp.go:connectMCPClient`.
  - Transport defaulting, streamable HTTP setup, stdio command setup, connection, and cleanup registration are mixed together. Separate transport creation would make connection lifecycle clearer.

- [x] Simplify `pkg/app/mcp.go:UpdateMCPServers` concurrency and URL handling.
  - Manual goroutine counting and channel collection can be replaced with a `sync.WaitGroup` plus mutex or an errgroup-style helper. URL building can use `strings.TrimRight(url, "/") + "/update"`.

- [x] Deduplicate enabled-default logic in `pkg/app/config.go`.
  - `MailConfig.IsEnabled`, `MatrixConfig.IsEnabled`, and `WebserverConfig.IsEnabled` all implement the same `*bool` default-true behavior. A small helper would remove repeated code.

- [x] Centralize assistant persona merging in `pkg/app/config.go`.
  - `configureAssistant`, `setDefaultPersona`, and `SetPersona` repeat field assignment/merge rules for `ai.AssistantConfig`. One helper for applying persona config would reduce drift.

- [x] Extract OpenAI client construction in `pkg/ai/openai.go`.
  - `GeneratePrompt` and `GenerateWithTools` both build request options, add optional base URL, and create a client. A `newOpenAIClient` helper would centralize this setup.

- [x] Extract Gemini client construction in `pkg/ai/gemini.go`.
  - `GeneratePrompt`, `GenerateWithTools`, `UploadFile`, `ListFiles`, and `DeleteFile` all call `genai.NewClient` with the same API key config. A helper would reduce boilerplate.

- [x] Break up `pkg/matrix/matrix.go:handleMessage`.
  - It handles room filtering, history cleanup, logging, read receipts, sender validation, attachment routing, router submission, and fallback response. Small guard/helper functions would make the message path easier to audit.

- [x] Break up `pkg/matrix/matrix.go:handleAttachment`.
  - The function performs URL selection, parsing, download, optional decrypt, validation, logging, upload, and user notices in one long flow. Extracting URL/download/decrypt/upload helpers would reduce branching.

- [x] Extract tool argument formatting from `pkg/matrix/matrix.go:calculateResponse`.
  - The threaded-tools notice builds and truncates argument strings inside the AI tool executor closure. A `formatToolArgs` helper would keep the closure focused on execution.

- [x] Simplify router tool definitions in `pkg/router/router.go:MCPTools`.
  - Inline JSON-schema maps make the function verbose. Package-level schema variables or tiny schema helpers would make tool registration easier to read.

- [x] Split channel destination formatting in `pkg/router/router.go:executeListChannelDestinations`.
  - Matrix and mail-specific detail formatting is nested inside a generic router function. Separate helpers per system would reduce conditionals and keep system-specific config access isolated.

- [x] Replace duplicated AI-name checks in `pkg/router/router.go:isAITarget` and `pkg/router/router.go:isAIAsync`.
  - The functions contain identical logic. Use one helper, likely based on `strings.EqualFold(target, "ai")` plus any intentionally supported aliases.

- [x] Share calendar result formatting in `pkg/mcp/ical/ical.go:handleListEvents` and `pkg/mcp/ical/ical.go:handleSearchEvents`.
  - Both handlers collect events, handle empty results, marshal JSON, and return MCP text content. A common result helper would remove repeated response construction.

- [x] Share ICS parsing/event construction in `pkg/mcp/ical/ical.go:getEvents` and `pkg/mcp/ical/ical.go:searchEvents`.
  - Both fetch/open/cache ICS files, configure a `gocal` parser, iterate events, filter, and construct `Event` values. Extracting parser setup and `Event` conversion would reduce duplication.

- [x] Move the Twizzit subscription handler body out of `pkg/mcp/twizzit/twizzit.go:registerGetSubscriptionByFormId`.
  - Tool registration currently contains query, response parsing, entry ID parsing, per-entry fetches, HTML stripping, and JSON response building inline. Named helpers would make failures and retries easier to reason about.

- [x] Add reusable MCP text-result helpers in modules such as `pkg/mcp/drillster/drillster.go` and `pkg/mcp/twizzit/twizzit_auth.go`.
  - Several request helpers repeat verbose `CallToolResult{Content: []TextContent{...}, IsError: ...}` blocks. A small `textResult(text string, isError bool)` helper would remove noise from error paths.

- [x] Split SMTP message construction from sending in `pkg/mail/imap.go:sendEmail`.
  - SMTP defaults, auth, subject fallback, MIME body construction, and send strategy are currently combined. Separate helpers for SMTP settings and MIME message building would make the function smaller.

- [x] Simplify recipient extraction in `pkg/mail/imap.go:extractRecipients`.
  - Address detection uses string contains/suffix checks and nested allowlist loops. `net/mail.ParseAddress` plus an allowlist map would make filtering clearer and less error-prone.
