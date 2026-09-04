// Package mwanachamacomm models chapter chat rooms, N-member direct/group
// threads, and message moderation for mwanachama-backend-api-gateway,
// storing them via GORM.
//
// Layout, mirroring mwanachama-backend-actor's split:
//   - models/    — domain types (ChatThread, DMThread, Report, ...) and the
//     repository interfaces (ChatRepository, DMRepository,
//     ModerationRepository) they're read and written through; callers use
//     models.ChatThread etc. directly, no re-export in this package
//   - gormstore/ — GORM row structs, row<->domain conversion, migration
//   - doc.go (this file), tables.go — table-name/migrate wrappers
//   - errors.go            — ErrInvalidReference/ErrConflict + classify()
//   - ids.go                — Clock, the one storage-agnostic helper left
//   - chat_impl.go, dm_impl.go, dm_message_impl.go, moderation_impl.go,
//     moderation_dismissal_impl.go — the GORM-backed store implementations
//   - routes/    — this package's own HTTP surface for the operations that
//     carry no gateway-only policy; see routes/doc.go for scope
//
// Ported from mwanachama-backend-api-gateway's internal/domain/{chat,
// directmessage,moderation} and internal/store/{postgres,memory}. See this
// repo's CLAUDE.md for what changed along the way, in particular why
// moderation's act-log write still doesn't import the gateway's custody
// package (unaffected by the storage swap this package went through
// 2026-09-04).
package mwanachamacomm
